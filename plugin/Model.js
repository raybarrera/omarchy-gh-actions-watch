// Pure helpers shared by BarWidget.qml and Panel.qml. Qt-free and locale-free
// so they can be unit tested under node (plugin/Model.test.mjs); the QML owns
// theming and layout.

function emptySnapshot() {
  return {
    account: "",
    host: "",
    generatedAt: "",
    running: [],
    queued: [],
    runningCount: 0,
    queuedCount: 0,
    activeCount: 0,
    apiCalls: 0,
    rateRemaining: -1,
    rateLimited: false,
    stale: false,
    pausedUntil: "",
    reposScanned: 0,
    durationSec: 0,
    error: ""
  }
}

function errorSnapshot(message) {
  var snap = emptySnapshot()
  snap.error = String(message || "unknown error")
  snap.generatedAt = new Date().toISOString()
  return snap
}

function parseSnapshot(text) {
  var raw = String(text || "").trim()
  if (raw === "") return emptySnapshot()
  var parsed
  try {
    parsed = JSON.parse(raw)
  } catch (e) {
    return errorSnapshot("could not read watcher output")
  }
  if (!parsed || typeof parsed !== "object") return errorSnapshot("empty watcher output")
  return normalize(parsed)
}

function normalize(snap) {
  var out = emptySnapshot()
  out.account = snap.account != null ? String(snap.account) : ""
  out.host = snap.host != null ? String(snap.host) : ""
  out.generatedAt = snap.generatedAt != null ? String(snap.generatedAt) : ""
  out.reposScanned = num(snap.reposScanned)
  out.durationSec = num(snap.durationSec)
  out.apiCalls = num(snap.apiCalls)
  out.rateRemaining = num(snap.rateRemaining)
  out.rateLimited = snap.rateLimited === true
  out.stale = snap.stale === true
  out.pausedUntil = str(snap.pausedUntil)
  out.error = snap.error != null ? String(snap.error) : ""
  out.running = runList(snap.running)
  out.queued = runList(snap.queued)
  out.runningCount = snap.runningCount != null ? num(snap.runningCount) : out.running.length
  out.queuedCount = snap.queuedCount != null ? num(snap.queuedCount) : out.queued.length
  out.activeCount = out.running.length + out.queued.length
  return out
}

function runList(value) {
  if (!value || !Array.isArray(value)) return []
  var out = []
  for (var i = 0; i < value.length; i++) {
    var run = value[i]
    if (!run || typeof run !== "object") continue
    out.push({
      repo: str(run.repo),
      workflow: str(run.workflow),
      runNumber: num(run.runNumber),
      branch: str(run.branch),
      event: str(run.event),
      status: str(run.status),
      conclusion: str(run.conclusion),
      actor: str(run.actor),
      displayTitle: str(run.displayTitle),
      startedAt: str(run.startedAt),
      createdAt: str(run.createdAt),
      url: str(run.url),
      elapsedSec: num(run.elapsedSec)
    })
  }
  return out
}

function str(value) {
  return value == null ? "" : String(value)
}

function num(value) {
  var n = Number(value)
  return isFinite(n) ? n : 0
}

function activeCount(snap) {
  if (!snap) return 0
  return (snap.running ? snap.running.length : 0) + (snap.queued ? snap.queued.length : 0)
}

function hasError(snap) {
  return !!(snap && snap.error && snap.error.length > 0)
}

function accountLabel(snap) {
  if (snap && snap.account && snap.account.length > 0) return snap.account
  return "your account"
}

function tooltip(snap) {
  var account = accountLabel(snap)
  if (hasError(snap)) return "GitHub Actions: " + snap.error
  var count = activeCount(snap)
  if (count === 0) return "No active Actions runs for " + account
  var running = snap.running ? snap.running.length : 0
  var queued = snap.queued ? snap.queued.length : 0
  var parts = []
  if (running > 0) parts.push(running + " running")
  if (queued > 0) parts.push(queued + " queued")
  return parts.join(", ") + " for " + account
}

function rows(snap) {
  var out = []
  if (!snap) return out
  if (snap.running) {
    for (var i = 0; i < snap.running.length; i++) out.push(withKind(snap.running[i], "running"))
  }
  if (snap.queued) {
    for (var j = 0; j < snap.queued.length; j++) out.push(withKind(snap.queued[j], "queued"))
  }
  return out
}

function withKind(run, kind) {
  var copy = {}
  for (var key in run) copy[key] = run[key]
  copy.kind = kind
  return copy
}

function shortRepo(fullName) {
  var s = str(fullName)
  var idx = s.indexOf("/")
  return idx >= 0 ? s.slice(idx + 1) : s
}

function owner(fullName) {
  var s = str(fullName)
  var idx = s.indexOf("/")
  return idx >= 0 ? s.slice(0, idx) : ""
}

function formatElapsed(seconds) {
  var s = Math.max(0, Math.floor(num(seconds)))
  if (s < 60) return s + "s"
  var m = Math.floor(s / 60)
  var remS = s % 60
  if (m < 60) return m + "m " + pad2(remS) + "s"
  var h = Math.floor(m / 60)
  var remM = m % 60
  return h + "h " + pad2(remM) + "m"
}

function elapsedSince(startIso, nowMs) {
  var start = Date.parse(str(startIso))
  if (isNaN(start)) return 0
  var diff = nowMs - start
  return diff > 0 ? Math.floor(diff / 1000) : 0
}

function runElapsed(run, nowMs) {
  if (!run) return 0
  var live = elapsedSince(run.startedAt || run.createdAt, nowMs)
  return live > 0 ? live : num(run.elapsedSec)
}

function formatClock(isoText) {
  var t = Date.parse(str(isoText))
  if (isNaN(t)) return ""
  var d = new Date(t)
  return pad2(d.getHours()) + ":" + pad2(d.getMinutes()) + ":" + pad2(d.getSeconds())
}

function detailLine(run) {
  if (!run) return ""
  var bits = []
  if (run.workflow) bits.push(str(run.workflow))
  if (run.branch) bits.push(str(run.branch))
  if (run.event) bits.push(str(run.event))
  return bits.join(" · ")
}

function metaLine(snap) {
  if (!snap) return ""
  var bits = []
  if (snap.reposScanned) bits.push(snap.reposScanned + " repos")
  if (snap.generatedAt) bits.push("updated " + formatClock(snap.generatedAt))
  return bits.join(" · ")
}

function gaugeCells(running, queued, cells) {
  var n = Math.max(1, Math.floor(num(cells)))
  var run = num(running)
  var queue = num(queued)
  var active = run + queue
  var out = []
  if (active <= 0) {
    for (var e = 0; e < n; e++) out.push("empty")
    return out
  }
  var runCells = Math.round(n * run / active)
  if (run > 0 && runCells < 1) runCells = 1
  var queueCells = n - runCells
  if (queue > 0 && queueCells < 1) {
    queueCells = 1
    runCells = n - 1
  }
  for (var r = 0; r < runCells; r++) out.push("run")
  for (var q = 0; q < queueCells; q++) out.push("queue")
  while (out.length < n) out.push("empty")
  return out.slice(0, n)
}

function statusLabel(kind) {
  return kind === "queued" ? "QUEUED" : "RUNNING"
}

function pad2(n) {
  return n < 10 ? "0" + n : String(n)
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = {
    emptySnapshot: emptySnapshot,
    errorSnapshot: errorSnapshot,
    parseSnapshot: parseSnapshot,
    normalize: normalize,
    activeCount: activeCount,
    hasError: hasError,
    accountLabel: accountLabel,
    tooltip: tooltip,
    rows: rows,
    detailLine: detailLine,
    shortRepo: shortRepo,
    owner: owner,
    formatElapsed: formatElapsed,
    elapsedSince: elapsedSince,
    runElapsed: runElapsed,
    formatClock: formatClock,
    metaLine: metaLine,
    gaugeCells: gaugeCells,
    statusLabel: statusLabel
  }
}
