package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/raybarrera/omarchy-gh-actions-watch/internal/ghactions"
)

func main() {
	var (
		account       = flag.String("account", "", "GitHub user or org to watch (defaults to the authenticated gh login)")
		host          = flag.String("host", "", "GitHub hostname for gh (defaults to the gh-configured host)")
		maxRepos      = flag.Int("max-repos", 50, "maximum number of repos to scan, most recently pushed first")
		timeout       = flag.Duration("timeout", 60*time.Second, "maximum time to spend querying GitHub")
		ghBin         = flag.String("gh", "gh", "path to the gh CLI binary")
		parallel      = flag.Int("parallel", 8, "number of repos to query concurrently")
		pretty        = flag.Bool("pretty", false, "indent the JSON output")
		cacheDir      = flag.String("cache-dir", "", "state dir for the ETag/repo cache (default: $XDG_STATE_HOME/omarchy/gh-actions-watch)")
		noCache       = flag.Bool("no-cache", false, "disable the on-disk cache (every poll re-fetches; costs rate limit)")
		repoTTL       = flag.Duration("repo-ttl", 15*time.Minute, "how long to reuse the cached repo list before re-listing")
		minRemaining  = flag.Int("min-remaining", 200, "pause polling when the GitHub rate-limit budget drops below this")
		includeQueued = flag.Bool("include-queued", false, "also query queued runs; doubles request count per repo")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "omarchy-gh-actions-watch - report active GitHub Actions runs for an account\n\n")
		fmt.Fprintf(os.Stderr, "Usage: omarchy-gh-actions-watch [flags]\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	dir := *cacheDir
	if *noCache {
		dir = ""
	} else if dir == "" {
		dir = ghactions.DefaultCacheDir()
	}

	runner := ghactions.NewRunner(*ghBin, *host)
	runner.Parallel = *parallel
	runner.RepoTTL = *repoTTL
	runner.MinRemaining = *minRemaining
	runner.Cache = ghactions.LoadCache(dir)

	snap := runner.Snapshot(ctx, ghactions.Options{
		Account:       *account,
		Host:          *host,
		MaxRepos:      *maxRepos,
		IncludeQueued: *includeQueued,
	})

	enc := json.NewEncoder(os.Stdout)
	if *pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(snap); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
}
