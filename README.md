# GitHub Actions Watch (Omarchy plugin)

An [Omarchy](https://omarchy.org/) Quattro **bar widget** that shows how many
GitHub Actions workflow runs are **in progress or queued right now** for a GitHub
account (user or org). Click the bar icon to open a panel listing each active run
with its repo, workflow, branch, event, elapsed time, and a link.

The GitHub API work lives in a small **Go** helper binary. The QML widget only runs
that helper and renders its JSON, following the same split Omarchy uses
for `omarchy.agents` and `omarchy.weather`.

Polling is lazy. Nothing contacts GitHub until you open the panel by hand. While the
panel stays open the helper re-runs every `refreshIntervalSec` so the listed runs are
followed. Closing the panel stops all polling.

```
 bar:   []  GitHub glyph + count chip  (2 = two active runs, ! = error)

 panel: ┌──────────────────────────────────────────────┐
        │  GITHUB ACTIONS                       [acct] │  filled title bar
        │  ACTIVE                    3 running · 1 queued│
        │  ▮▮▮▮▮▮▮▮▮▮▮▮▯▯▯▯▯▯▯▯▯▯▯▯▯▯▯  ← cell gauge      │  running vs queued
        │ ────────────────────────────────────────────  │
        │ [RUNNING]  web                    [ 3m 12s ]  │  status + elapsed chips
        │  CI · main · push                              │
        │  #7  Fix the thing                             │
        │  ▓▓▓▓▓▓░░░░░░░░░░  ← sweeping activity bar     │
        │                                                │
        │ [QUEUED]   api                    [ 0s ]       │
        │  Deploy · dev · workflow_dispatch              │
        │  ░░░░░░░░░░░░░░░░  ← pulsing bar               │
        │  2 repos · updated 00:39            [ Refresh ]│
        └──────────────────────────────────────────────┘
```

## Look & feel

A Charm/bubbletea-flavoured TUI look built from theme tokens (monospace, uppercase
letterspaced labels, lipgloss-style chips and a filled title bar) and block-cell
meters drawn with `Rectangle`s so nothing depends on which glyphs the font ships.

Progress visuals, and what is honest to draw from the data:

- **Cell gauge** (determinate): the running-vs-queued split of active runs, from real
  counts. This is the only true meter, because the API gives no completion total.
- **Activity bar per run** (indeterminate): a sweep for `in_progress`, a pulse for
  `queued`. An active run's record carries no total duration, so there is no honest
  per-run percentage; the motion says "working" or "waiting" without faking progress.
- **Status / elapsed chips** and an **ALL CLEAR** badge replace the old text rows.

A future enhancement could fetch each run's jobs/steps to compute real step-completion
progress; that costs extra API calls per active run and is intentionally not done here.

## How it works

- `omarchy-gh-actions-watch` (Go) queries GitHub through the **already-authenticated
  `gh` CLI** (`gh api -i`). It never stores or reads tokens itself.
- It makes **one Actions API call per repository**, requesting up to 100 latest runs
  and locally filtering active states. Repositories are listed once per account per
  30 minutes, then cached on disk.
- Each account's cache stores run-query ETags and response bodies across polls; the
  cache lives under `$XDG_STATE_HOME/omarchy/gh-actions-watch/` (fallback:
  `~/.local/state/omarchy/gh-actions-watch/`). Cache files are created private (0600).
- Repo enumeration is cached for 30 minutes. Requests are serialized, one per repo,
  with a 60-second interval that runs only while the panel is open and a 30-repo default cap. The helper parses
  rate-limit headers, stops below 200 remaining (or on 403/429), and serves the last
  cached snapshot until the reset time.
- **Rate-limit caveat:** conditional requests reduce response body transfer and return
  cached data, but GitHub's `X-RateLimit-Remaining` can still decrement on some 304s.
  These are still real requests; no-cache is even more expensive. Don't shorten the
  interval casually. `apiCalls` in the helper JSON counts non-304 responses only; check
  `rateRemaining` for the server-reported budget.

## Dependencies

- **Omarchy** with the Quattro shell (the plugin shares the long-running
  `omarchy-shell` process; it does not start a second Quickshell).
- **`gh`** CLI, installed and authenticated (`gh auth login`). This is the only
  network/auth path.
- **Go** (to build the helper) and the plugin's `Model.js` tests need **Node**.

## Install

```sh
git clone <this repo> && cd omarchy-gh-actions-watch
make install      # builds the Go helper into ~/.local/bin and the plugin into
                  # ~/.config/omarchy/plugins/io.github.raybarrera.gh-actions-watch
omarchy-shell shell rescanPlugins
omarchy plugin enable io.github.raybarrera.gh-actions-watch --section right
```

`make install` puts the helper at `~/.local/bin/omarchy-gh-actions-watch` (make sure
that directory is on your `PATH`, which the shell inherits).

If you change the QML after the shell already loaded it, a plain rescan may keep the
old compiled component; run `omarchy restart shell` to be sure the new QML is loaded.

## Configure

Settings are stored per-widget in `~/.config/omarchy/shell.json` and can be set with
`omarchy bar set`:

| Key | Default | Meaning |
|-----|---------|---------|
| `account` | `""` (your `gh` login) | GitHub user or org login to watch |
| `host` | `""` (github.com) | GitHub Enterprise hostname passed to `gh --hostname` |
| `refreshIntervalSec` | `60` | Poll interval while the panel is open. Nothing is polled when it is closed. |
| `maxRepos` | `30` | Most-recently-pushed repos to scan; reduces first-poll cost. |

```sh
omarchy bar set io.github.raybarrera.gh-actions-watch account your-org --json
omarchy bar set io.github.raybarrera.gh-actions-watch refreshIntervalSec 120 --json
```

## Use

- **Left click** the bar icon: open/close the panel. Opening it checks GitHub; the badge
  shows the last result and is empty until the first open.
- **Right click**: refresh immediately.
- **Click a run row**: open that run on GitHub.
- **Escape**: close the panel.
- Shell routes: `omarchy-shell shell summon io.github.raybarrera.gh-actions-watch '{}'`
  and `omarchy-shell shell hide io.github.raybarrera.gh-actions-watch`.

## Develop

```sh
make build    # build the Go helper into ./bin
make test     # go test ./... and the Model.js node tests
make lint     # go vet + qmllint
make validate # omarchy plugin validate plugin
```

Layout:

```
cmd/omarchy-gh-actions-watch/   Go CLI (flags: -account -host -max-repos -timeout -gh -parallel -pretty)
internal/ghactions/             GitHub querying + snapshot model (unit tested with a fake gh)
plugin/                         manifest.json, BarWidget.qml, Panel.qml, Model.js
test/Model.test.cjs             node tests for the QML-side Model.js helpers
```

Note on `qmllint`: the documented `qmllint -I "$OMARCHY_PATH/shell"` invocation cannot
resolve the Quickshell and `qs.Ui`/`qs.Commons` modules in a plain shell (it exits
non-zero even on Omarchy's own first-party plugins). The authoritative check is
loading the plugin in the running shell and reading `qs log -p "$OMARCHY_PATH/shell"`
for QML errors.

## Remove

```sh
omarchy plugin disable io.github.raybarrera.gh-actions-watch
make uninstall   # removes the helper and the plugin folder
```

## Namespace & privacy

The plugin id `io.github.raybarrera.gh-actions-watch`, the Go module path, and the
manifest `author` carry the author's GitHub handle. That handle is required by the
Omarchy third-party id convention (`io.github.<user>.<plugin>`) and is public by
nature; nothing else identifying is stored in the repo. The watcher reads no tokens
(auth is delegated to `gh`), and the project `.gitignore` excludes the local session
data (`.crush/`) and build output (`bin/`). To publish under a different namespace,
rename the id in `plugin/manifest.json`, the `moduleName` in `BarWidget.qml` and
`Panel.qml`, `PLUGIN_ID` in the `Makefile`, and the module path in `go.mod`.

## License

MIT. See [LICENSE](LICENSE).
