package ghactions

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- fake gh -------------------------------------------------------------

type fakeRoute struct {
	etag      string
	body      string
	status    int // 0 means 200
	remaining int // 0 means a healthy budget
}

type fakeGH struct {
	mu        sync.Mutex
	routes    map[string]fakeRoute
	realCalls int // responses that were NOT 304, i.e. that consumed rate limit
}

func transcript(status int, etag string, remaining int, body string) []byte {
	text := map[int]string{200: "OK", 304: "Not Modified", 403: "Forbidden", 404: "Not Found", 429: "Too Many Requests"}[status]
	if text == "" {
		text = "Status"
	}
	if remaining == 0 {
		remaining = 4990
	}
	var b strings.Builder
	fmt.Fprintf(&b, "HTTP/2.0 %d %s\r\n", status, text)
	if etag != "" {
		fmt.Fprintf(&b, "Etag: %s\r\n", etag)
	}
	fmt.Fprintf(&b, "X-Ratelimit-Remaining: %d\r\n", remaining)
	b.WriteString("X-Ratelimit-Reset: 9999999999\r\n")
	b.WriteString("Content-Type: application/json\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return []byte(b.String())
}

func parseArgs(args []string) (endpoint, inm string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-H" && i+1 < len(args):
			h := args[i+1]
			if v, ok := strings.CutPrefix(h, "If-None-Match:"); ok {
				inm = strings.TrimSpace(v)
			}
			i++
		case a == "--hostname":
			i++
		case a == "api" || a == "-i":
		case strings.HasPrefix(a, "/"):
			endpoint = a
		}
	}
	return
}

func (f *fakeGH) exec() ExecFunc {
	return func(ctx context.Context, args []string) ([]byte, int, error) {
		endpoint, inm := parseArgs(args)
		f.mu.Lock()
		defer f.mu.Unlock()

		keys := make([]string, 0, len(f.routes))
		for k := range f.routes {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

		for _, k := range keys {
			if !containsAll(endpoint, strings.Fields(k)) {
				continue
			}
			route := f.routes[k]
			status := route.status
			if status == 0 {
				status = 200
			}
			if status != 200 {
				f.realCalls++
				return transcript(status, route.etag, route.remaining, route.body), 1, nil
			}
			if inm != "" && inm == route.etag {
				return transcript(304, route.etag, route.remaining, ""), 1, nil
			}
			f.realCalls++
			return transcript(200, route.etag, route.remaining, route.body), 0, nil
		}
		f.realCalls++
		return transcript(404, "", 4990, `{"message":"Not Found"}`), 1, nil
	}
}

func containsAll(endpoint string, parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	for _, p := range parts {
		if !strings.Contains(endpoint, p) {
			return false
		}
	}
	return true
}

// ---- fixtures ------------------------------------------------------------

var fixedNow = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func startedAgo(d time.Duration) string {
	return fixedNow.Add(-d).Format(time.RFC3339)
}

func twoRepoRoutes() map[string]fakeRoute {
	started := startedAgo(90 * time.Second)
	repos := `[{"full_name":"acme/web","pushed_at":"2026-10-07T10:00:00Z"},{"full_name":"acme/api","pushed_at":"2026-10-06T10:00:00Z"}]`
	inProgress := `{"total_count":2,"workflow_runs":[{"id":11,"name":"CI","workflow_id":1,"run_number":5,"status":"in_progress","head_branch":"main","event":"push","display_title":"fix thing","html_url":"https://github.com/acme/web/actions/runs/11","created_at":"` + started + `","run_started_at":"` + started + `","actor":{"login":"octo"}}]}`
	queued := `{"total_count":1,"workflow_runs":[{"id":22,"name":"Deploy","workflow_id":2,"run_number":3,"status":"queued","head_branch":"dev","event":"workflow_dispatch","html_url":"https://github.com/acme/api/actions/runs/22","created_at":"` + started + `","actor":{"login":"octo"}}]}`
	return map[string]fakeRoute{
		"/orgs/acme/repos":                         {etag: `W/"repos"`, body: repos},
		"acme/web/actions/runs status=in_progress": {etag: `W/"web-in"`, body: inProgress},
		"acme/web/actions/runs status=queued":      {etag: `W/"web-q"`, body: `{"total_count":0,"workflow_runs":[]}`},
		"acme/api/actions/runs status=in_progress": {etag: `W/"api-in"`, body: `{"total_count":0,"workflow_runs":[]}`},
		"acme/api/actions/runs status=queued":      {etag: `W/"api-q"`, body: queued},
	}
}

func newRunner(t *testing.T, f *fakeGH, dir string) *Runner {
	t.Helper()
	r := NewRunner("gh", "")
	r.Exec = f.exec()
	r.Now = func() time.Time { return fixedNow }
	r.Cache = LoadCache(dir)
	return r
}

// ---- tests ---------------------------------------------------------------

func TestSnapshotCollectsActiveRuns(t *testing.T) {
	f := &fakeGH{routes: twoRepoRoutes()}
	r := newRunner(t, f, t.TempDir())

	snap := r.Snapshot(context.Background(), Options{Account: "acme", IncludeQueued: true})
	if snap.Error != "" {
		t.Fatalf("unexpected error: %s", snap.Error)
	}
	if snap.RunningCount != 1 || snap.QueuedCount != 1 || snap.ActiveCount != 2 {
		t.Fatalf("counts = %d/%d/%d, want 1/1/2", snap.RunningCount, snap.QueuedCount, snap.ActiveCount)
	}
	if snap.ReposScanned != 2 {
		t.Fatalf("reposScanned = %d, want 2", snap.ReposScanned)
	}
	if got := snap.Running[0].ElapsedSec; got < 85 || got > 95 {
		t.Fatalf("elapsedSec = %d, want ~90", got)
	}
	if snap.Running[0].Actor != "octo" || snap.Queued[0].Repo != "acme/api" {
		t.Fatalf("unexpected runs: %+v / %+v", snap.Running[0], snap.Queued[0])
	}
}

// one conditional request per repo; the repo list is cached. ETags keep the
// body transfer small, but 304s are still requests and may consume budget.
func TestSecondPollUsesConditionalRequests(t *testing.T) {
	dir := t.TempDir()
	f := &fakeGH{routes: twoRepoRoutes()}

	r1 := newRunner(t, f, dir)
	snap1 := r1.Snapshot(context.Background(), Options{Account: "acme", IncludeQueued: true})
	if snap1.ActiveCount != 2 {
		t.Fatalf("first poll active = %d, want 2", snap1.ActiveCount)
	}
	first := f.realCalls
	if first == 0 {
		t.Fatal("first poll should make real calls")
	}

	// Simulate the next one-shot invocation: a fresh Runner over the same cache.
	r2 := newRunner(t, f, dir)
	snap2 := r2.Snapshot(context.Background(), Options{Account: "acme", IncludeQueued: true})
	delta := f.realCalls - first

	if delta != 0 {
		t.Fatalf("second poll made %d real calls, want 0 (304s only)", delta)
	}
	if snap2.ActiveCount != 2 {
		t.Fatalf("second poll active = %d, want 2 (served from 304 cache)", snap2.ActiveCount)
	}
	if snap2.APICalls != 4 {
		t.Fatalf("second poll APICalls = %d, want 4 conditional requests for two statuses across two repos", snap2.APICalls)
	}
}

func TestRateLimitPausesAndServesStale(t *testing.T) {
	dir := t.TempDir()
	routes := twoRepoRoutes()
	low := routes["acme/web/actions/runs"]
	low.remaining = 5 // below MinRemaining -> pause
	routes["acme/web/actions/runs status=in_progress"] = low

	f := &fakeGH{routes: routes}
	r1 := newRunner(t, f, dir)
	r1.MinRemaining = 50
	snap1 := r1.Snapshot(context.Background(), Options{Account: "acme"})
	if !snap1.RateLimited {
		t.Fatal("expected RateLimited after a low-remaining response")
	}

	before := f.realCalls
	r2 := newRunner(t, f, dir)
	r2.MinRemaining = 50
	snap2 := r2.Snapshot(context.Background(), Options{Account: "acme"})
	if f.realCalls != before {
		t.Fatalf("paused poll made %d calls, want 0", f.realCalls-before)
	}
	if !snap2.Stale || !snap2.RateLimited {
		t.Fatalf("paused poll should be stale+rateLimited: %+v", snap2)
	}
	if snap2.PausedUntil == "" {
		t.Fatal("paused poll should report PausedUntil")
	}
}

func TestAccountErrorSurfacesInSnapshot(t *testing.T) {
	f := &fakeGH{routes: map[string]fakeRoute{
		"/orgs/ghost/repos":  {status: 404, body: `{"message":"Not Found"}`},
		"/users/ghost/repos": {status: 404, body: `{"message":"Not Found"}`},
	}}
	r := newRunner(t, f, t.TempDir())

	snap := r.Snapshot(context.Background(), Options{Account: "ghost"})
	if snap.Error == "" || !strings.Contains(snap.Error, "ghost") {
		t.Fatalf("expected an account error mentioning ghost, got %q", snap.Error)
	}
	if snap.ActiveCount != 0 {
		t.Fatalf("activeCount = %d, want 0", snap.ActiveCount)
	}
}

func TestDefaultsToAuthenticatedLogin(t *testing.T) {
	repos := `[{"full_name":"me/thing","pushed_at":"2026-10-07T10:00:00Z"}]`
	empty := `{"total_count":0,"workflow_runs":[]}`
	f := &fakeGH{routes: map[string]fakeRoute{
		"/user":                 {etag: `W/"user"`, body: `{"login":"me","type":"User"}`},
		"/orgs/me/repos":        {status: 404, body: `{"message":"Not Found"}`},
		"/user/repos":           {etag: `W/"merepos"`, body: repos},
		"me/thing/actions/runs": {etag: `W/"thing"`, body: empty},
	}}
	r := newRunner(t, f, t.TempDir())

	snap := r.Snapshot(context.Background(), Options{})
	if snap.Error != "" {
		t.Fatalf("unexpected error: %s", snap.Error)
	}
	if snap.Account != "me" {
		t.Fatalf("account = %q, want me", snap.Account)
	}
}

func TestSelectAndCapRepos(t *testing.T) {
	repos := []apiRepo{
		{FullName: "a", PushedAt: "2026-01-01T00:00:00Z"},
		{FullName: "b", PushedAt: "2026-06-01T00:00:00Z"},
		{FullName: "c", PushedAt: "2026-03-01T00:00:00Z"},
	}
	got := capRepos(selectRepos(repos), 2)
	if len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Fatalf("got %v, want [b c]", got)
	}
}

func TestParseGHResponse(t *testing.T) {
	raw := transcript(200, `W/"abc"`, 4321, `{"total_count":0,"workflow_runs":[]}`)
	resp := parseGHResponse(raw)
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200", resp.Status)
	}
	if resp.ETag != `W/"abc"` {
		t.Fatalf("etag = %q", resp.ETag)
	}
	if resp.Remaining != 4321 {
		t.Fatalf("remaining = %d, want 4321", resp.Remaining)
	}
	var rp runsPage
	if err := json.Unmarshal(resp.Body, &rp); err != nil {
		t.Fatalf("body not json: %v (%q)", err, resp.Body)
	}
}

func TestParseGHResponse304HasNoBody(t *testing.T) {
	resp := parseGHResponse(transcript(304, `W/"abc"`, 4990, ""))
	if resp.Status != 304 {
		t.Fatalf("status = %d, want 304", resp.Status)
	}
	if len(resp.Body) != 0 {
		t.Fatalf("304 body should be empty, got %q", resp.Body)
	}
}
