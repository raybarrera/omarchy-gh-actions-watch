# Work Order: omarchy-gh-actions-watch

## Kind
Change behavior (greenfield): a new Omarchy Quattro bar-widget plugin backed by a
Go helper binary that reports currently running/queued GitHub Actions workflow runs
for a specified GitHub account (user or org).

## Problem
There is no at-a-glance view in the Omarchy bar of in-flight CI. The user wants a
bar widget that shows how many GitHub Actions runs are active right now for a given
account, and a panel listing each active run (repo, workflow, branch, elapsed, link).

## Constraints / findings
- Plugins share the long-running `omarchy-shell` Quickshell process; never spawn a
  second Quickshell. External work is done by a helper binary invoked via
  `Quickshell.Io.Process` (pattern used by `omarchy.agents` and `omarchy.weather`).
- Org-level endpoint `/orgs/{org}/actions/runs` returns 404 with the installed token
  scopes (`read:org`, `repo`, `workflow`). Repo-level
  `/repos/{owner}/{repo}/actions/runs?status=...` works universally. So the watcher
  enumerates the account's repos and queries each one.
- Auth/transport is delegated to the installed, already-authenticated `gh` CLI
  (`gh api`). No token handling in the plugin; avoids storing secrets.
- Pure Go stdlib, no external modules, so `go build` needs no network.

## Architecture
- Go binary `omarchy-gh-actions-watch` (installed on PATH): one-shot `snapshot`
  prints a single JSON document to stdout describing active runs.
  - Resolves account (defaults to the authenticated `gh` login).
  - Lists repos (`/user/repos`, `/orgs/{o}/repos`, or `/users/{u}/repos`), sorted by
    most recently pushed, capped at `-max-repos`.
  - Queries each repo for `in_progress` and `queued` runs concurrently.
  - Emits `{account, generatedAt, running[], queued[], counts, error}`.
- QML bar-widget plugin `io.github.raybarrera.gh-actions-watch`:
  - `BarWidget.qml` shows the active-run count; `Loader` hosts `Panel.qml`.
  - `Panel.qml` runs the binary on a Timer, collects stdout, renders the list.
  - `Model.js` parses/derives display values; `manifest.json` declares settings
    (account, host, refresh interval, max repos).

## Done when (checkable)
1. `go build ./...` and `go vet ./...` pass; `go test ./...` passes.
2. The binary produces valid JSON for a real account against live `gh`
   (`runningCount`/`queuedCount` present, no error).
3. The binary handles a bad account with a JSON `error` field and exit 0 (so the
   panel can render a message rather than crash).
4. `omarchy plugin validate <plugin-folder>` exits 0.
5. `qmllint -I "$OMARCHY_PATH/shell" BarWidget.qml Panel.qml` reports no errors
   for the plugin's own QML.
6. README documents install, the `gh` dependency, and configuration.

## Out of scope
- Notifications on run completion (future enhancement).
- Historical/finished-run browsing; only currently active runs.
- Non-`gh` auth paths (GHE via `gh --hostname` is supported but not separately tested).
