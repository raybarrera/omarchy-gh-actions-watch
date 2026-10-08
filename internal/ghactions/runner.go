package ghactions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ExecFunc runs gh and returns stdout, exit code, and a spawn error. Nonzero
// status is expected for `gh api -i` on 304/4xx; parse stdout for HTTP status.
type ExecFunc func(ctx context.Context, args []string) (stdout []byte, exitCode int, err error)

type Runner struct {
	GhBin        string
	Host         string
	Exec         ExecFunc
	Now          func() time.Time
	Parallel     int
	Cache        *CacheStore
	RepoTTL      time.Duration
	MinRemaining int
	mu           sync.Mutex
	calls        int
	remaining    int
	reset        int64
	paused       bool
	rateGate     chan struct{}
}

func NewRunner(ghBin, host string) *Runner {
	return &Runner{GhBin: ghBin, Host: host, Exec: defaultExec(ghBin), Now: time.Now, Parallel: 1, RepoTTL: 30 * time.Minute, MinRemaining: 200, remaining: -1, rateGate: make(chan struct{}, 1)}
}

func defaultExec(ghBin string) ExecFunc {
	return func(ctx context.Context, args []string) ([]byte, int, error) {
		cmd := exec.CommandContext(ctx, ghBin, args...)
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				return []byte(stdout.String()), 0, err
			}
		}
		out := stdout.String()
		if code != 0 && !strings.HasPrefix(out, "HTTP/") {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			return []byte(out), code, errors.New(msg)
		}
		return []byte(out), code, nil
	}
}

func (r *Runner) Snapshot(ctx context.Context, opts Options) Snapshot {
	started := r.Now()
	snap := Snapshot{Account: opts.Account, Host: r.Host, GeneratedAt: started.UTC().Format(time.RFC3339), Running: []Run{}, Queued: []Run{}, RateRemaining: -1}
	if opts.MaxRepos <= 0 {
		opts.MaxRepos = 30
	}
	if r.Parallel <= 0 {
		r.Parallel = 4
	}
	if r.Cache == nil {
		r.Cache = LoadCache("")
	}
	if r.Cache.paused(started) {
		return r.servePaused(&snap, started)
	}

	account := strings.TrimSpace(opts.Account)
	if account == "" {
		login, err := r.resolveLogin(ctx)
		if err != nil {
			snap.Error = err.Error()
			return r.finish(&snap, started)
		}
		account = login
	}
	snap.Account = account
	key := strings.ToLower(account)
	acct := r.Cache.account(key)
	repos, err := r.listRepos(ctx, account, key, &acct, opts.MaxRepos)
	if err != nil {
		snap.Error = err.Error()
		return r.finish(&snap, started)
	}
	snap.ReposScanned = len(repos)

	runs, _ := r.collectRuns(ctx, repos, &acct, started, opts.IncludeQueued)
	for _, run := range runs {
		switch run.Status {
		case "queued", "requested", "waiting", "pending":
			snap.Queued = append(snap.Queued, run)
		default:
			snap.Running = append(snap.Running, run)
		}
	}
	snap.RunningCount, snap.QueuedCount = len(snap.Running), len(snap.Queued)
	snap.ActiveCount = snap.RunningCount + snap.QueuedCount
	acct.Last = &snap
	r.Cache.setAccount(key, acct)
	return r.finish(&snap, started)
}

func (r *Runner) servePaused(snap *Snapshot, started time.Time) Snapshot {
	snap.RateLimited, snap.Stale = true, true
	if pause := r.Cache.pauseUntil(); !pause.IsZero() {
		snap.PausedUntil = pause.UTC().Format(time.RFC3339)
	}
	if last := r.cachedLast(snap.Account); last != nil {
		out := *last
		out.RateLimited, out.Stale = true, true
		out.PausedUntil = snap.PausedUntil
		out.DurationSec, out.APICalls = 0, 0
		return r.finish(&out, started)
	}
	snap.Error = "rate limited by GitHub; waiting for the limit to reset"
	return r.finish(snap, started)
}

func (r *Runner) cachedLast(account string) *Snapshot {
	key := strings.ToLower(strings.TrimSpace(account))
	if key == "" {
		key = strings.ToLower(r.Cache.data.Login)
	}
	return r.Cache.account(key).Last
}

func (r *Runner) finish(snap *Snapshot, started time.Time) Snapshot {
	r.mu.Lock()
	snap.APICalls = r.calls
	if r.remaining >= 0 {
		snap.RateRemaining = r.remaining
	}
	snap.RateLimited = snap.RateLimited || r.paused
	r.mu.Unlock()
	snap.DurationSec = int64(r.Now().Sub(started).Seconds())
	if snap.RateLimited {
		if pause := r.Cache.pauseUntil(); !pause.IsZero() {
			snap.PausedUntil = pause.UTC().Format(time.RFC3339)
		}
	}
	snap.Stale = snap.RateLimited
	r.Cache.Save()
	return *snap
}

func (r *Runner) resolveLogin(ctx context.Context) (string, error) {
	if r.Cache.data.Login != "" {
		return r.Cache.data.Login, nil
	}
	out, err := r.getJSON(ctx, "/user", "")
	if err != nil {
		return "", fmt.Errorf("gh auth: %w", err)
	}
	var u apiUser
	if err := json.Unmarshal(out, &u); err != nil {
		return "", fmt.Errorf("gh auth: %w", err)
	}
	if u.Login == "" {
		return "", errors.New("gh auth: no authenticated user")
	}
	r.Cache.data.Login = u.Login
	return u.Login, nil
}

func (r *Runner) do(ctx context.Context, endpoint, etag string) (ghResponse, error) {
	if r.rateGate != nil {
		select {
		case r.rateGate <- struct{}{}:
			defer func() { <-r.rateGate }()
		case <-ctx.Done():
			return ghResponse{Remaining: -1}, ctx.Err()
		}
	}
	args := []string{"api", "-i"}
	if etag != "" {
		args = append(args, "-H", "If-None-Match: "+etag)
	}
	args = append(args, endpoint, "-H", "Accept: application/vnd.github+json")
	if r.Host != "" {
		args = append(args, "--hostname", r.Host)
	}
	out, _, err := r.Exec(ctx, args)
	if err != nil {
		return ghResponse{Remaining: -1}, err
	}
	resp := parseGHResponse(out)
	if resp.Status == 0 {
		return resp, errors.New("unexpected gh output")
	}
	r.observeRate(resp)
	return resp, nil
}

func (r *Runner) observeRate(resp ghResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if resp.Remaining >= 0 {
		r.remaining = resp.Remaining
		r.Cache.data.Remaining = resp.Remaining
	}
	if resp.Reset > 0 {
		r.reset = resp.Reset
	}
	if resp.Status == 403 || resp.Status == 429 || (resp.Remaining >= 0 && resp.Remaining < r.MinRemaining) {
		until := r.reset
		if resp.RetryAfter > 0 {
			until = r.Now().Add(time.Duration(resp.RetryAfter) * time.Second).Unix()
		}
		if until <= r.Now().Unix() {
			until = r.Now().Add(5 * time.Minute).Unix()
		}
		r.paused = true
		r.Cache.data.PauseUntil = until
	}
}

func (r *Runner) getJSON(ctx context.Context, endpoint, etag string) ([]byte, error) {
	resp, err := r.do(ctx, endpoint, etag)
	if err != nil {
		return nil, err
	}
	switch resp.Status {
	case 200:
		return resp.Body, nil
	case 304:
		return nil, nil
	default:
		return nil, fmt.Errorf("gh: HTTP %d", resp.Status)
	}
}

func (r *Runner) listRepos(ctx context.Context, account, key string, acct *accountCache, maxRepos int) ([]string, error) {
	if len(acct.Repos) > 0 && acct.ReposFetched > 0 && r.Now().Unix()-acct.ReposFetched < int64(r.RepoTTL/time.Second) {
		return capRepos(acct.Repos, maxRepos), nil
	}
	path := acct.ReposPath
	if path == "" {
		path = fmt.Sprintf("/orgs/%s/repos", account)
	}
	repos, err := r.paginateRepos(ctx, path)
	if err != nil {
		alt := fmt.Sprintf("/users/%s/repos", account)
		if strings.EqualFold(account, r.Cache.data.Login) {
			alt = "/user/repos"
		}
		if alt == path {
			return nil, fmt.Errorf("list repos for %s: %w", account, err)
		}
		repos, err = r.paginateRepos(ctx, alt)
		if err != nil {
			return nil, fmt.Errorf("list repos for %s: %w", account, err)
		}
		path = alt
	}
	names := selectRepos(repos)
	acct.ReposPath, acct.Repos, acct.ReposFetched = path, names, r.Now().Unix()
	r.Cache.setAccount(key, *acct)
	return capRepos(names, maxRepos), nil
}

func (r *Runner) paginateRepos(ctx context.Context, path string) ([]apiRepo, error) {
	var all []apiRepo
	for page := 1; page <= 20; page++ {
		q := url.Values{}
		q.Set("per_page", "100")
		q.Set("page", strconv.Itoa(page))
		q.Set("sort", "pushed")
		out, err := r.getJSON(ctx, path+"?"+q.Encode(), "")
		if err != nil {
			return nil, err
		}
		var batch []apiRepo
		if err := json.Unmarshal(out, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return all, nil
}

func selectRepos(repos []apiRepo) []string {
	sort.SliceStable(repos, func(i, j int) bool { return repos[i].PushedAt > repos[j].PushedAt })
	out := make([]string, 0, len(repos))
	for _, repo := range repos {
		if repo.FullName != "" {
			out = append(out, repo.FullName)
		}
	}
	return out
}

func capRepos(repos []string, maxRepos int) []string {
	if maxRepos > 0 && len(repos) > maxRepos {
		return repos[:maxRepos]
	}
	return repos
}

func (r *Runner) collectRuns(ctx context.Context, repos []string, acct *accountCache, now time.Time, includeQueued bool) ([]Run, []Run) {
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		runs []Run
	)
	for _, repo := range repos {
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			items := r.repoRunsStatus(ctx, repo, "in_progress", acct, &mu, now)
			if includeQueued {
				items = append(items, r.repoRunsStatus(ctx, repo, "queued", acct, &mu, now)...)
			}
			mu.Lock()
			runs = append(runs, items...)
			mu.Unlock()
		}(repo)
	}
	wg.Wait()
	sortRuns(runs)
	return runs, nil
}

func sortRuns(runs []Run) {
	sort.SliceStable(runs, func(i, j int) bool {
		if runs[i].Repo != runs[j].Repo {
			return runs[i].Repo < runs[j].Repo
		}
		return runs[i].StartedAt > runs[j].StartedAt
	})
}

func (r *Runner) repoRunsStatus(ctx context.Context, repo, status string, acct *accountCache, mu *sync.Mutex, now time.Time) []Run {
	q := url.Values{}
	q.Set("status", status)
	q.Set("per_page", "100")
	endpoint := fmt.Sprintf("/repos/%s/actions/runs", repo) + "?" + q.Encode()
	mu.Lock()
	cached := acct.Entries[endpoint]
	mu.Unlock()
	resp, err := r.do(ctx, endpoint, cached.ETag)
	if err != nil {
		return nil
	}
	var body []byte
	switch resp.Status {
	case 304:
		body = []byte(cached.Body)
	case 200:
		body = resp.Body
		mu.Lock()
		if acct.Entries == nil {
			acct.Entries = map[string]entry{}
		}
		acct.Entries[endpoint] = entry{ETag: resp.ETag, Body: string(resp.Body)}
		mu.Unlock()
	default:
		return nil
	}
	var rp runsPage
	if len(body) == 0 || json.Unmarshal(body, &rp) != nil {
		return nil
	}
	out := make([]Run, 0, len(rp.WorkflowRuns))
	for _, ar := range rp.WorkflowRuns {
		out = append(out, r.convert(repo, ar, now))
	}
	return out
}

func (r *Runner) convert(repo string, ar apiRun, now time.Time) Run {
	run := Run{Repo: repo, Workflow: ar.Name, WorkflowID: ar.WorkflowID, RunID: ar.ID, RunNumber: ar.RunNumber, Branch: ar.HeadBranch, Event: ar.Event, Status: ar.Status, DisplayTitle: ar.DisplayTitle, CreatedAt: ar.CreatedAt, UpdatedAt: ar.UpdatedAt, URL: ar.HTMLURL}
	if ar.Conclusion != nil {
		run.Conclusion = *ar.Conclusion
	}
	if ar.Actor != nil {
		run.Actor = ar.Actor.Login
	}
	start := ar.CreatedAt
	if ar.RunStartedAt != nil && *ar.RunStartedAt != "" {
		start = *ar.RunStartedAt
		run.StartedAt = *ar.RunStartedAt
	} else {
		run.StartedAt = ar.CreatedAt
	}
	if t, err := time.Parse(time.RFC3339, start); err == nil && now.After(t) {
		run.ElapsedSec = int64(now.Sub(t).Seconds())
	}
	return run
}
