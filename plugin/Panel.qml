import QtQuick
import Quickshell
import qs.Commons
import qs.Ui
import "Model.js" as Model

// Popup for the GitHub Actions Watch bar widget. Charm/bubbletea-flavoured: a
// filled title bar, a block-cell gauge for the running/queued split, and one
// spacious card per active run with an animated activity bar. Reads the latest
// snapshot off the host bar widget (which owns the polling process).
//
// The activity bars are deliberately indeterminate: an active run's API record
// carries no total duration, so there is no honest completion percentage to
// draw. The sweep says "working", the pulse says "waiting", and the only
// determinate meter is the running-vs-queued gauge, which is real counts.
Panel {
  id: root
  moduleName: "io.github.raybarrera.gh-actions-watch"
  manageIpc: false

  property var anchorItem: null
  property var hostWidget: null

  readonly property var snapshot: hostWidget ? hostWidget.snapshot : Model.emptySnapshot()
  readonly property var rowList: Model.rows(snapshot)
  readonly property color foreground: bar ? bar.barForeground : Color.foreground
  readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

  readonly property color runColor: Color.accent
  readonly property color queueColor: Color.muted
  readonly property color alertColor: Color.urgent
  readonly property color dimText: Qt.darker(foreground, 1.5)
  readonly property color faintText: Qt.darker(foreground, 1.9)
  readonly property color trackColor: Qt.alpha(foreground, 0.10)
  readonly property color hairline: Qt.alpha(foreground, 0.12)

  readonly property int gaugeN: 30

  // Elapsed times tick while the panel is open instead of freezing at the last
  // poll, so a long run keeps counting up.
  property double nowMs: Date.now()

  function open() {
    root.controller.show()
    if (hostWidget && typeof hostWidget.refresh === "function") hostWidget.refresh()
  }

  Timer {
    interval: 1000
    running: root.opened
    repeat: true
    onTriggered: root.nowMs = Date.now()
  }

  KeyboardPanel {
    id: panel
    anchorItem: root.anchorItem
    owner: root.hostWidget || root
    bar: root.bar
    open: root.opened
    focusTarget: keyCatcher
    contentWidth: panel.fittedContentWidth(Style.space(460))
    contentHeight: panel.fittedContentHeight(content.implicitHeight)

    PanelKeyCatcher {
      id: keyCatcher
      anchors.fill: parent
      onCloseRequested: root.close()
      onTabRequested: function(direction) { root.switchPanel(direction) }

      Column {
        id: content
        width: parent.width
        spacing: Style.space(16)

        // ---- title bar (lipgloss-style filled header)
        Rectangle {
          id: headerBar
          width: parent.width
          height: Style.space(38)
          radius: Style.space(9)
          color: Style.selectedFill

          Text {
            id: headerIcon
            anchors.left: parent.left
            anchors.leftMargin: Style.space(13)
            anchors.verticalCenter: parent.verticalCenter
            textFormat: Text.PlainText
            text: "\uf09b"
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.title
          }

          Text {
            id: headerTitle
            anchors.left: headerIcon.right
            anchors.leftMargin: Style.space(9)
            anchors.verticalCenter: parent.verticalCenter
            textFormat: Text.PlainText
            text: "GITHUB ACTIONS"
            color: root.foreground
            font.family: root.fontFamily
            font.pixelSize: Style.font.subtitle
            font.bold: true
            font.letterSpacing: Style.space(1)
          }

          Rectangle {
            id: accountChip
            anchors.right: parent.right
            anchors.rightMargin: Style.space(10)
            anchors.verticalCenter: parent.verticalCenter
            width: accountText.implicitWidth + Style.space(16)
            height: Style.space(22)
            radius: height / 2
            color: "transparent"
            border.width: 1
            border.color: root.hairline

            Text {
              id: accountText
              anchors.centerIn: parent
              textFormat: Text.PlainText
              text: Model.accountLabel(root.snapshot)
              color: root.dimText
              font.family: root.fontFamily
              font.pixelSize: Style.font.caption
              font.bold: true
            }
          }
        }

        // ---- active gauge: the one determinate meter (running vs queued)
        Column {
          id: gaugeBlock
          width: parent.width
          spacing: Style.space(9)
          visible: !Model.hasError(root.snapshot)

          Item {
            width: parent.width
            height: rateNotice.implicitHeight + Style.space(12)
            visible: root.snapshot.stale

            Rectangle {
              anchors.fill: parent
              radius: Style.space(6)
              color: Qt.alpha(root.queueColor, 0.12)
              border.width: 1
              border.color: Qt.alpha(root.queueColor, 0.30)
            }

            Text {
              id: rateNotice
              anchors.left: parent.left
              anchors.right: parent.right
              anchors.verticalCenter: parent.verticalCenter
              anchors.leftMargin: Style.space(10)
              anchors.rightMargin: Style.space(10)
              textFormat: Text.PlainText
              text: root.snapshot.rateLimited
                ? "Rate limit reached. Showing cached data until " + (root.snapshot.pausedUntil || "reset")
                : "Showing last cached data while GitHub is unavailable."
              color: root.queueColor
              font.family: root.fontFamily
              font.pixelSize: Style.font.caption
              wrapMode: Text.WordWrap
            }
          }

          Item {
            width: parent.width
            height: gaugeLabel.implicitHeight

            Text {
              id: gaugeLabel
              anchors.left: parent.left
              anchors.verticalCenter: parent.verticalCenter
              textFormat: Text.PlainText
              text: "ACTIVE"
              color: root.faintText
              font.family: root.fontFamily
              font.pixelSize: Style.font.caption
              font.bold: true
              font.letterSpacing: Style.space(2)
            }

            Row {
              id: gaugeCounts
              anchors.right: parent.right
              anchors.verticalCenter: parent.verticalCenter
              spacing: Style.space(8)

              Text {
                anchors.verticalCenter: parent.verticalCenter
                textFormat: Text.PlainText
                text: (root.snapshot.runningCount || 0) + " running"
                color: root.runColor
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.bold: true
              }

              Text {
                anchors.verticalCenter: parent.verticalCenter
                textFormat: Text.PlainText
                text: "·"
                color: root.faintText
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
              }

              Text {
                anchors.verticalCenter: parent.verticalCenter
                textFormat: Text.PlainText
                text: (root.snapshot.queuedCount || 0) + " queued"
                color: root.queueColor
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                font.bold: true
              }
            }
          }

          Row {
            id: gaugeRow
            width: parent.width
            height: Style.space(12)
            spacing: Style.space(2)

            Repeater {
              model: Model.gaugeCells(root.snapshot.runningCount, root.snapshot.queuedCount, root.gaugeN)

              Rectangle {
                required property var modelData
                width: (gaugeRow.width - gaugeRow.spacing * (root.gaugeN - 1)) / root.gaugeN
                height: gaugeRow.height
                radius: Style.space(2)
                color: modelData === "run" ? root.runColor
                  : modelData === "queue" ? root.queueColor
                  : root.trackColor
              }
            }
          }
        }

        Rectangle {
          width: parent.width
          height: 1
          color: root.hairline
          visible: !Model.hasError(root.snapshot)
        }

        // ---- error card
        Rectangle {
          id: errorCard
          width: parent.width
          visible: Model.hasError(root.snapshot)
          height: errorRow.implicitHeight + Style.space(24)
          radius: Style.space(9)
          color: Qt.alpha(root.alertColor, 0.12)
          border.width: 1
          border.color: Qt.alpha(root.alertColor, 0.45)

          Row {
            id: errorRow
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: Style.space(14)
            anchors.rightMargin: Style.space(14)
            spacing: Style.space(10)

            Text {
              anchors.verticalCenter: parent.verticalCenter
              textFormat: Text.PlainText
              text: "\uf071"
              color: root.alertColor
              font.family: root.fontFamily
              font.pixelSize: Style.font.title
            }

            Text {
              width: parent.width - Style.space(40)
              text: root.snapshot.error || ""
              color: root.alertColor
              font.family: root.fontFamily
              font.pixelSize: Style.font.bodySmall
              wrapMode: Text.WordWrap
            }
          }
        }

        // ---- empty state
        Rectangle {
          id: emptyCard
          width: parent.width
          visible: !Model.hasError(root.snapshot) && root.rowList.length === 0
          height: Style.space(120)
          radius: Style.space(9)
          color: root.trackColor

          Column {
            anchors.centerIn: parent
            spacing: Style.space(10)

            Rectangle {
              anchors.horizontalCenter: parent.horizontalCenter
              width: Style.space(46)
              height: Style.space(46)
              radius: width / 2
              color: "transparent"
              border.width: 2
              border.color: root.runColor

              Text {
                anchors.centerIn: parent
                textFormat: Text.PlainText
                text: "\uf00c"
                color: root.runColor
                font.family: root.fontFamily
                font.pixelSize: Style.font.heading
              }
            }

            Text {
              anchors.horizontalCenter: parent.horizontalCenter
              textFormat: Text.PlainText
              text: "ALL CLEAR"
              color: root.foreground
              font.family: root.fontFamily
              font.pixelSize: Style.font.subtitle
              font.bold: true
              font.letterSpacing: Style.space(2)
            }

            Text {
              anchors.horizontalCenter: parent.horizontalCenter
              textFormat: Text.PlainText
              text: "Nothing running or queued"
              color: root.dimText
              font.family: root.fontFamily
              font.pixelSize: Style.font.bodySmall
            }
          }
        }

        // ---- run cards
        ListView {
          id: list
          width: parent.width
          visible: root.rowList.length > 0
          model: root.rowList
          spacing: Style.space(12)
          clip: true
          interactive: height < contentHeight
          height: Math.min(contentHeight, Style.space(420))

          delegate: Rectangle {
            id: card
            required property var modelData
            width: list.width
            height: cardCol.implicitHeight + Style.space(26)
            radius: Style.space(9)
            color: cardMa.containsMouse ? Style.hoverFill : Style.normalFill
            border.width: 1
            border.color: cardMa.containsMouse ? Style.hoverBorderColor : root.hairline

            Column {
              id: cardCol
              anchors.left: parent.left
              anchors.right: parent.right
              anchors.verticalCenter: parent.verticalCenter
              anchors.leftMargin: Style.space(15)
              anchors.rightMargin: Style.space(15)
              spacing: Style.space(9)

              Item {
                width: parent.width
                height: Math.max(statusChip.height, repoText.implicitHeight, elapsedChip.height)

                Rectangle {
                  id: statusChip
                  anchors.left: parent.left
                  anchors.verticalCenter: parent.verticalCenter
                  width: statusText.implicitWidth + Style.space(16)
                  height: Style.space(22)
                  radius: Style.space(5)
                  color: Qt.alpha(card.modelData.kind === "queued" ? root.queueColor : root.runColor, 0.16)
                  border.width: 1
                  border.color: Qt.alpha(card.modelData.kind === "queued" ? root.queueColor : root.runColor, 0.55)

                  Text {
                    id: statusText
                    anchors.centerIn: parent
                    textFormat: Text.PlainText
                    text: Model.statusLabel(card.modelData.kind)
                    color: card.modelData.kind === "queued" ? root.queueColor : root.runColor
                    font.family: root.fontFamily
                    font.pixelSize: Style.font.caption
                    font.bold: true
                    font.letterSpacing: Style.space(1)
                  }
                }

                Text {
                  id: repoText
                  anchors.left: statusChip.right
                  anchors.leftMargin: Style.space(10)
                  anchors.right: elapsedChip.left
                  anchors.rightMargin: Style.space(10)
                  anchors.verticalCenter: parent.verticalCenter
                  textFormat: Text.PlainText
                  text: Model.shortRepo(card.modelData.repo)
                  elide: Text.ElideMiddle
                  color: root.foreground
                  font.family: root.fontFamily
                  font.pixelSize: Style.font.body
                  font.bold: true
                }

                Rectangle {
                  id: elapsedChip
                  anchors.right: parent.right
                  anchors.verticalCenter: parent.verticalCenter
                  width: elapsedText.implicitWidth + Style.space(14)
                  height: Style.space(22)
                  radius: Style.space(5)
                  color: "transparent"
                  border.width: 1
                  border.color: root.hairline

                  Text {
                    id: elapsedText
                    anchors.centerIn: parent
                    textFormat: Text.PlainText
                    text: Model.formatElapsed(Model.runElapsed(card.modelData, root.nowMs))
                    color: root.dimText
                    font.family: root.fontFamily
                    font.pixelSize: Style.font.caption
                    font.bold: true
                  }
                }
              }

              Text {
                width: parent.width
                textFormat: Text.PlainText
                text: Model.detailLine(card.modelData)
                elide: Text.ElideRight
                color: root.dimText
                font.family: root.fontFamily
                font.pixelSize: Style.font.bodySmall
              }

              Text {
                width: parent.width
                visible: card.modelData.displayTitle && card.modelData.displayTitle.length > 0
                textFormat: Text.PlainText
                text: "#" + card.modelData.runNumber + "  " + card.modelData.displayTitle
                elide: Text.ElideRight
                color: root.faintText
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
              }

              // indeterminate activity bar: sweep while running, pulse while queued
              Rectangle {
                id: actTrack
                width: parent.width
                height: Style.space(7)
                radius: height / 2
                color: root.trackColor
                clip: true

                Rectangle {
                  id: actFill
                  height: parent.height
                  radius: height / 2
                  color: card.modelData.kind === "queued" ? root.queueColor : root.runColor
                  width: card.modelData.kind === "queued" ? actTrack.width : actTrack.width * 0.34
                  x: 0
                  opacity: card.modelData.kind === "queued" ? 0.5 : 1.0

                  NumberAnimation on x {
                    running: root.opened && card.modelData.kind === "running"
                    from: -actTrack.width * 0.34
                    to: actTrack.width
                    duration: 1500
                    loops: Animation.Infinite
                    easing.type: Easing.InOutQuad
                  }

                  NumberAnimation on opacity {
                    running: root.opened && card.modelData.kind === "queued"
                    from: 0.22
                    to: 0.6
                    duration: 1000
                    loops: Animation.Infinite
                    easing.type: Easing.InOutSine
                  }
                }
              }
            }

            MouseArea {
              id: cardMa
              anchors.fill: parent
              hoverEnabled: true
              cursorShape: card.modelData.url ? Qt.PointingHandCursor : Qt.ArrowCursor
              onClicked: if (card.modelData.url) Qt.openUrlExternally(card.modelData.url)
            }
          }
        }

        // ---- footer
        Item {
          width: parent.width
          height: Math.max(metaText.implicitHeight, refreshBtn.height)

          Text {
            id: metaText
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            textFormat: Text.PlainText
            text: "API remaining " + (root.snapshot.rateRemaining >= 0 ? root.snapshot.rateRemaining : "?")
              + " · " + (root.snapshot.apiCalls || 0) + " calls this poll"
              + (root.snapshot.stale ? " · CACHED" : "")
            color: root.faintText
            font.family: root.fontFamily
            font.pixelSize: Style.font.caption
          }

          Button {
            id: refreshBtn
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            text: "Refresh + queued"
            iconText: "\uf021"
            focusable: true
            foreground: root.foreground
            onClicked: if (root.hostWidget && typeof root.hostWidget.refreshQueued === "function") root.hostWidget.refreshQueued()
          }
        }
      }
    }
  }
}
