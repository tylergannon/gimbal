import SwiftUI

struct RecorderControls: View {
  @ObservedObject var controller: RecorderController

  var body: some View {
    HStack(alignment: .center, spacing: 12) {
      switch controller.status.state {
      case "recording":
        RecordingButton(
          title: "Pause", symbol: "pause.fill", isRecord: false, action: controller.pause
        )
        .disabled(!controller.canPause)
      case "paused":
        if controller.canResume {
          RecordingButton(
            title: "Resume", symbol: "play.fill", isRecord: false, action: controller.resume)
        } else {
          waitingControl("Please wait…")
        }
      case "preparing", "pausing", "resuming", "stopping":
        waitingControl(progressLabel)
      default:
        if !controller.canStop {
          if controller.canRecord {
            RecordingButton(
              title: "Record", symbol: "circle.fill", isRecord: true, action: controller.record)
          } else {
            waitingControl("Please wait…")
          }
        }
      }
      if controller.canStop && controller.status.state != "stopping" {
        Button(action: controller.stop) {
          HStack(spacing: 6) {
            Image(systemName: "stop.fill")
              .font(.system(size: 19, weight: .semibold))
              .foregroundStyle(.red)
              .frame(width: 20, height: 20)
              .background(.background, in: Circle())
              .overlay(Circle().stroke(.secondary.opacity(0.25), lineWidth: 1))
              .frame(width: 24, height: 24)
            Text("Stop").font(.subheadline.weight(.medium)).lineLimit(1)
          }
          .padding(8)
          .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Stop")
        .help("End recording and keep the transcript")
      }
    }
    .padding(.horizontal, 24)
  }

  private func waitingControl(_ label: String) -> some View {
    HStack(spacing: 6) {
      ProgressView().controlSize(.small).frame(width: 24, height: 24)
      Text(label).font(.subheadline).foregroundStyle(.secondary).lineLimit(1)
    }
    .padding(8)
    .accessibilityElement(children: .combine)
  }

  private var progressLabel: String {
    switch controller.status.state {
    case "preparing": "Preparing…"
    case "pausing": "Pausing…"
    case "resuming": "Resuming…"
    default: "Finishing…"
    }
  }
}

private struct RecordingButton: View {
  let title: String
  let symbol: String
  let isRecord: Bool
  let action: () -> Void

  var body: some View {
    Button(action: action) {
      HStack(spacing: 6) {
        Image(systemName: symbol)
          .font(.system(size: isRecord ? 18 : 16, weight: .semibold))
          .foregroundStyle(isRecord ? Color.red : Color.primary)
          .frame(width: 24, height: 24)
          .background(.background, in: Circle())
          .overlay(
            Circle().stroke(
              isRecord ? Color.red.opacity(0.8) : Color.secondary.opacity(0.25),
              lineWidth: isRecord ? 2 : 1))
        Text(title).font(.subheadline.weight(.medium)).lineLimit(1)
      }
      .padding(8)
      .contentShape(Rectangle())
    }
    .buttonStyle(.plain)
    .accessibilityLabel(title)
    .help(
      title == "Pause"
        ? "Pause this recording"
        : title == "Resume" ? "Continue this recording" : "Start a new recording")
  }
}

struct RecordingTimer: View {
  @ObservedObject var controller: RecorderController
  var body: some View {
    TimelineView(.periodic(from: .now, by: 1)) { _ in
      let seconds = Int(controller.recordingDuration.elapsed())
      let time =
        seconds >= 3600
        ? String(format: "%d:%02d:%02d", seconds / 3600, (seconds / 60) % 60, seconds % 60)
        : String(format: "%02d:%02d", seconds / 60, seconds % 60)
      HStack(spacing: 6) {
        Circle().fill(controller.status.state == "recording" ? Color.red : Color.secondary)
          .frame(width: 6, height: 6)
        Text(time).font(.subheadline.monospacedDigit())
      }
      .foregroundStyle(.secondary)
      .accessibilityElement(children: .ignore)
      .accessibilityLabel("Elapsed recording time \(time)")
    }
  }
}

// Five horizontal bars fill from the bottom upward, without adding a status row.
struct RecordingLevelMeter: View {
  @ObservedObject var controller: RecorderController
  @Environment(\.accessibilityReduceMotion) private var reduceMotion

  private var activeBars: Int { Int(ceil(controller.audioLevel * 5)) }
  var body: some View {
    VStack(spacing: 2) {
      ForEach(0..<5) { index in
        RoundedRectangle(cornerRadius: 1.5)
          .fill(Color.green.opacity(4 - index < activeBars ? 1 : 0.18))
          .frame(width: 20, height: 3)
      }
    }
    .frame(width: 20, height: 23)
    .animation(reduceMotion ? nil : .easeOut(duration: 0.1), value: activeBars)
    .accessibilityElement(children: .ignore)
    .accessibilityLabel(
      controller.status.source == "synthetic" ? "Simulated audio level" : "Microphone level"
    )
    .accessibilityValue("\(activeBars) of 5")
  }
}

#if os(macOS)
  import AppKit
  import Combine

  @MainActor final class RecordingStatusItem: NSObject, ObservableObject {
    let controller: RecorderController
    let input: RecordingInput
    let openTranscript: () -> Void
    @Published private(set) var freshness = "Waiting for the first saved words"
    private let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
    private let popover = NSPopover()
    private var subscription: AnyCancellable?
    private var timer: Timer?
    private var savedSequence = 0
    private var savedSession = ""
    private var savedAt: Date?

    init(
      controller: RecorderController, input: RecordingInput, openTranscript: @escaping () -> Void
    ) {
      self.controller = controller
      self.input = input
      self.openTranscript = openTranscript
      super.init()
      item.button?.target = self
      item.button?.action = #selector(togglePopover)
      item.button?.setAccessibilityLabel("Gimbal recording controls")
      item.button?.toolTip = "Gimbal recording controls"
      popover.behavior = .transient
      popover.contentViewController = NSHostingController(
        rootView: RecordingStatusPopover(status: self))
      subscription = controller.objectWillChange.sink { [weak self] _ in
        DispatchQueue.main.async { self?.refresh() }
      }
      timer = Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { [weak self] _ in
        MainActor.assumeIsolated { self?.refresh() }
      }
      refresh()
    }

    private func refresh() {
      let status = controller.status
      if status.sessionID != savedSession {
        savedSession = status.sessionID
        savedSequence = 0
        savedAt = nil
      }
      if !status.pending && status.sequence > savedSequence {
        savedSequence = status.sequence
        savedAt = Date()
      }
      let nextFreshness =
        status.pending
        ? "Saving transcript…"
        : savedAt.map { "Last words saved \(Int(Date().timeIntervalSince($0))) seconds ago" }
          ?? "Waiting for the first saved words"
      if freshness != nextFreshness { freshness = nextFreshness }
      guard let button = item.button else { return }
      if controller.canStop {
        let seconds = Int(controller.recordingDuration.elapsed())
        let time =
          seconds >= 3600
          ? String(format: "%d:%02d:%02d", seconds / 3600, (seconds / 60) % 60, seconds % 60)
          : String(format: "%02d:%02d", seconds / 60, seconds % 60)
        let title = NSMutableAttributedString(string: "● \(time)")
        title.addAttribute(
          .foregroundColor,
          value: status.state == "recording" ? NSColor.systemRed : NSColor.secondaryLabelColor,
          range: NSRange(location: 0, length: 1))
        button.image = nil
        button.attributedTitle = title
        button.setAccessibilityValue("\(status.state), elapsed \(time)")
      } else {
        button.title = "Gimbal"
        button.image = NSImage(systemSymbolName: "mic", accessibilityDescription: "Recording")
        button.imagePosition = .imageLeading
        button.setAccessibilityValue(status.state)
      }
    }

    @objc private func togglePopover() {
      if popover.isShown {
        popover.close()
      } else {
        showControls()
      }
    }

    func showControls() {
      if let button = item.button {
        popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
      }
    }

    func showTranscript() {
      popover.close()
      openTranscript()
    }

    func shutdown() {
      timer?.invalidate()
      subscription?.cancel()
      popover.close()
      NSStatusBar.system.removeStatusItem(item)
    }
  }

  private struct RecordingStatusPopover: View {
    @ObservedObject var status: RecordingStatusItem
    @ObservedObject var controller: RecorderController
    @ObservedObject var input: RecordingInput

    init(status: RecordingStatusItem) {
      self.status = status
      controller = status.controller
      input = status.input
    }

    var body: some View {
      VStack(alignment: .leading, spacing: 14) {
        HStack {
          Text("Gimbal Recording").font(.headline)
          Spacer()
          if controller.canStop {
            RecordingLevelMeter(controller: controller)
            RecordingTimer(controller: controller)
          }
        }
        RecordingInputNotice(input: input)
        if controller.canStop {
          Text(status.freshness).font(.caption).foregroundStyle(.secondary)
        }
        if let error = controller.status.error {
          Text(error).font(.caption).foregroundStyle(.red)
        }
        RecorderControls(controller: controller)
          .frame(maxWidth: .infinity)
        Divider()
        Button("Open Transcript", action: status.showTranscript)
          .disabled(controller.status.sessionID.isEmpty)
        SettingsLink { Text("Recording Settings…") }
      }
      .padding(16)
      .frame(width: 340)
    }
  }
#endif
