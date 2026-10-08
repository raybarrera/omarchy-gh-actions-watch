import QtQuick
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui
import "Model.js" as Model

// GitHub Actions activity for one account. A GitHub glyph sits in the bar with
// a count badge of the runs that are in progress or queued right now; left
// click opens the panel listing them.
//
// All GitHub access lives in the omarchy-gh-actions-watch helper (a Go binary
// that talks to the API through the authenticated gh CLI). This file only runs
// that helper on a timer and renders its JSON, mirroring how omarchy.agents and
// omarchy.weather shell out to their collectors.
BarWidget {
  id: root
  moduleName: "io.github.raybarrera.gh-actions-watch"

  property var snapshot: Model.emptySnapshot()

  readonly property int activeCount: Model.activeCount(snapshot)
  readonly property bool hasError: Model.hasError(snapshot)

  readonly property string account: String(setting("account", "") || "")
  readonly property string host: String(setting("host", "") || "")
  readonly property int refreshSec: Math.max(60, parseInt(setting("refreshIntervalSec", 300), 10) || 300)
  readonly property int maxRepos: Math.max(1, parseInt(setting("maxRepos", 30), 10) || 30)
  readonly property int includeQueued: setting("includeQueued", false) ? 1 : 0

  function buildCommand() {
    var cmd = ["omarchy-gh-actions-watch", "-max-repos", String(root.maxRepos)]
    if (root.includeQueued) cmd.push("-include-queued")
    if (root.account !== "") cmd.push("-account", root.account)
    if (root.host !== "") cmd.push("-host", root.host)
    return cmd
  }

  function refresh() {
    if (poll.running) return
    poll.command = root.buildCommand()
    poll.running = true
  }

  function refreshQueued() {
    if (poll.running) return
    var cmd = root.buildCommand()
    cmd.push("-include-queued")
    poll.command = cmd
    poll.running = true
  }

  // ---- Panel lifecycle. Bar.findPanelWidget requires open/close/opened on the
  //      bar-widget root, forwarded to the loaded panel.
  readonly property bool opened: panelLoader.item ? panelLoader.item.opened === true : false

  function open() {
    if (panelLoader.item) panelLoader.item.open()
    root.refresh()
  }

  function close() {
    if (panelLoader.item) panelLoader.item.close()
  }

  function toggle() {
    if (root.opened) root.close()
    else root.open()
  }

  readonly property bool popoutSwitchClosing: panelLoader.item ? panelLoader.item.popoutSwitchClosing === true : false

  function closeForPopoutSwitch() {
    if (panelLoader.item) panelLoader.item.closeForPopoutSwitch()
  }

  function injectPanel() {
    var target = panelLoader.item
    if (!target) return
    if ("bar" in target) target.bar = root.bar
    if ("settings" in target) target.settings = root.settings
    if ("anchorItem" in target) target.anchorItem = button
    if ("hostWidget" in target) target.hostWidget = root
  }

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  onBarChanged: root.injectPanel()
  onSettingsChanged: {
    root.injectPanel()
    root.refresh()
  }

  Process {
    id: poll
    running: false
    stdout: StdioCollector {
      waitForEnd: true
      onStreamFinished: root.snapshot = Model.parseSnapshot(text)
    }
    onExited: function(exitCode) {
      if (exitCode !== 0 && root.activeCount === 0 && !root.hasError)
        root.snapshot = Model.errorSnapshot("watcher exited with code " + exitCode)
    }
  }

  Timer {
    interval: root.refreshSec * 1000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: root.refresh()
  }

  IpcHandler {
    target: root.moduleName

    function refresh(): void { root.broadcast("refresh") }
    function open(): void { root.open() }
    function close(): void { root.close() }
    function show(): void { root.open() }
    function hide(): void { root.close() }
    function toggle(): void { root.toggle() }
  }

  Loader {
    id: panelLoader
    active: true
    source: Qt.resolvedUrl("Panel.qml")
    visible: false
    onLoaded: {
      root.injectPanel()
      Qt.callLater(root.injectPanel)
    }
  }

  BarIconButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    text: "\uf09b"
    slotSize: Style.bar.statusSlot
    fontSize: Style.font.caption
    active: root.activeCount > 0
    useActiveColor: true
    tooltipText: Model.tooltip(root.snapshot)
    onPressed: function(b) {
      if (b === Qt.RightButton) root.refreshQueued()
      else root.toggle()
    }

    Rectangle {
      id: badge
      visible: root.activeCount > 0 || root.hasError
      readonly property color badgeColor: root.hasError ? Color.urgent : Color.accent
      width: count.implicitWidth + Style.space(7)
      height: count.implicitHeight + Style.space(3)
      radius: height / 2
      color: badgeColor
      anchors.right: parent.right
      anchors.top: parent.top
      anchors.rightMargin: -Style.space(2)
      anchors.topMargin: -Style.space(1)

      Text {
        id: count
        anchors.centerIn: parent
        textFormat: Text.PlainText
        text: root.snapshot.rateLimited ? "~" : (root.hasError ? "!" : String(root.activeCount))
        color: Color.background
        font.family: root.bar ? root.bar.fontFamily : Style.font.family
        font.pixelSize: Style.font.caption - 1
        font.bold: true
        renderType: Text.NativeRendering
      }
    }
  }
}
