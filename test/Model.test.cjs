const test = require("node:test")
const assert = require("node:assert")
const Model = require("../plugin/Model.js")

test("parseSnapshot normalizes counts and arrays", () => {
  const raw = JSON.stringify({
    account: "acme",
    generatedAt: "2026-10-07T12:00:00Z",
    reposScanned: 2,
    running: [{ repo: "acme/web", workflow: "CI", branch: "main", event: "push", status: "in_progress", startedAt: "2026-10-07T11:59:00Z", url: "https://x", runNumber: 5, displayTitle: "fix" }],
    queued: [{ repo: "acme/api", workflow: "Deploy", branch: "dev", event: "dispatch", status: "queued", createdAt: "2026-10-07T11:59:30Z", url: "https://y", runNumber: 3 }]
  })
  const snap = Model.parseSnapshot(raw)
  assert.strictEqual(snap.error, "")
  assert.strictEqual(snap.account, "acme")
  assert.strictEqual(snap.running.length, 1)
  assert.strictEqual(snap.queued.length, 1)
  assert.strictEqual(Model.activeCount(snap), 2)
})

test("parseSnapshot survives garbage input", () => {
  const snap = Model.parseSnapshot("not json at all")
  assert.ok(Model.hasError(snap))
  assert.strictEqual(Model.activeCount(snap), 0)
})

test("empty input yields a clean empty snapshot", () => {
  const snap = Model.parseSnapshot("")
  assert.strictEqual(snap.error, "")
  assert.deepStrictEqual(Model.rows(snap), [])
})

test("rows tags running before queued", () => {
  const snap = Model.normalize({
    running: [{ repo: "a/b", status: "in_progress" }],
    queued: [{ repo: "c/d", status: "queued" }]
  })
  const rows = Model.rows(snap)
  assert.strictEqual(rows.length, 2)
  assert.strictEqual(rows[0].kind, "running")
  assert.strictEqual(rows[1].kind, "queued")
})

test("formatElapsed covers seconds, minutes, and hours", () => {
  assert.strictEqual(Model.formatElapsed(45), "45s")
  assert.strictEqual(Model.formatElapsed(125), "2m 05s")
  assert.strictEqual(Model.formatElapsed(3725), "1h 02m")
})

test("runElapsed prefers live time over the stored value", () => {
  const now = Date.parse("2026-10-07T12:00:00Z")
  const run = { startedAt: "2026-10-07T11:58:00Z", elapsedSec: 1 }
  assert.strictEqual(Model.runElapsed(run, now), 120)
})

test("shortRepo and detailLine shape the row copy", () => {
  assert.strictEqual(Model.shortRepo("acme/web"), "web")
  assert.strictEqual(Model.detailLine({ workflow: "CI", branch: "main", event: "push" }), "CI · main · push")
})

test("tooltip reflects count and account", () => {
  const snap = Model.normalize({ account: "acme", running: [{ repo: "a" }], queued: [{ repo: "b" }] })
  assert.strictEqual(Model.tooltip(snap), "1 running, 1 queued for acme")
  assert.strictEqual(Model.tooltip(Model.emptySnapshot()), "No active Actions runs for your account")
})

test("account defaults to a friendly label", () => {
  assert.strictEqual(Model.accountLabel(Model.emptySnapshot()), "your account")
  assert.strictEqual(Model.accountLabel({ account: "acme" }), "acme")
})

test("gaugeCells is all empty when nothing is active", () => {
  const cells = Model.gaugeCells(0, 0, 12)
  assert.strictEqual(cells.length, 12)
  assert.ok(cells.every((c) => c === "empty"))
})

test("gaugeCells splits running vs queued proportionally", () => {
  const cells = Model.gaugeCells(3, 1, 8)
  assert.strictEqual(cells.filter((c) => c === "run").length, 6)
  assert.strictEqual(cells.filter((c) => c === "queue").length, 2)
})

test("gaugeCells keeps at least one cell per non-zero kind", () => {
  const cells = Model.gaugeCells(1, 9, 10)
  assert.strictEqual(cells.filter((c) => c === "run").length, 1)
  assert.strictEqual(cells.filter((c) => c === "queue").length, 9)
})

test("gaugeCells is all run when nothing is queued", () => {
  const cells = Model.gaugeCells(2, 0, 10)
  assert.strictEqual(cells.length, 10)
  assert.ok(cells.every((c) => c === "run"))
})

test("statusLabel maps kind to the chip copy", () => {
  assert.strictEqual(Model.statusLabel("queued"), "QUEUED")
  assert.strictEqual(Model.statusLabel("running"), "RUNNING")
  assert.strictEqual(Model.statusLabel(undefined), "RUNNING")
})
