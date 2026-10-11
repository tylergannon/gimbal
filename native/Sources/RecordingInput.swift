import AppleTranscription
import SwiftUI

@MainActor final class RecordingInput: ObservableObject {
  @Published var selectedUID = ""
  @Published private(set) var devices: [InputDevice] = []
  @Published private(set) var error: String?
  let fixtureURL: URL?
  let configurationError: String?
  private let listInputs: @MainActor () throws -> [InputDevice]

  var selection: InputDeviceSelection {
    selectedUID.isEmpty ? .systemDefault : .device(uid: selectedUID)
  }

  var isSelectionUnavailable: Bool {
    !selectedUID.isEmpty && !devices.contains { $0.uid == selectedUID }
  }

  init(
    environment: [String: String] = ProcessInfo.processInfo.environment,
    bundle: Bundle = .main,
    listInputs: @escaping @MainActor () throws -> [InputDevice] = InputDeviceInventory.inputs
  ) {
    self.listInputs = listInputs
    let source =
      environment["VN_SOURCE"] ?? bundle.object(forInfoDictionaryKey: "VNDefaultSource") as? String
      ?? "real"
    if let path = environment["VN_TRANSCRIPT_FIXTURE"] {
      fixtureURL = URL(fileURLWithPath: path)
      configurationError = nil
    } else if source == "synthetic" {
      let fixtureName =
        bundle.object(forInfoDictionaryKey: "VNTranscriptFixture") as? String
        ?? "synthetic-transcript"
      if fixtureName.isEmpty || fixtureName.contains("/") || fixtureName.contains("\\")
        || fixtureName == "." || fixtureName == ".." || fixtureName.hasSuffix(".json")
      {
        fixtureURL = nil
        configurationError =
          "Invalid synthetic transcript fixture. VNTranscriptFixture must name a bundled JSON resource without a path or extension."
      } else {
        // A missing resource remains an explicit source error, never a microphone fallback.
        fixtureURL =
          bundle.url(forResource: fixtureName, withExtension: "json")
          ?? bundle.bundleURL.appendingPathComponent("missing-\(fixtureName).json")
        configurationError = nil
      }
    } else if source == "real" {
      fixtureURL = nil
      configurationError = nil
    } else {
      fixtureURL = nil
      configurationError =
        "Invalid transcript source. Set VN_SOURCE to real or synthetic, or provide VN_TRANSCRIPT_FIXTURE."
    }
    selectedUID = environment["VN_INPUT_DEVICE"] ?? ""
    error = configurationError
    if fixtureURL == nil && configurationError == nil { refresh() }
  }

  func refresh() {
    guard fixtureURL == nil, configurationError == nil else { return }
    do {
      devices = try listInputs()
      error = nil
    } catch { self.error = error.localizedDescription }
  }
}

struct RecordingInputNotice: View {
  @ObservedObject var input: RecordingInput
  var body: some View {
    if let error = input.configurationError {
      Text(error).font(.caption).foregroundStyle(.red)
    } else if input.fixtureURL != nil {
      Label("Synthetic transcript · microphone off", systemImage: "waveform.path")
        .font(.caption).foregroundStyle(.secondary)
    } else if let error = input.error {
      Text(error).font(.caption).foregroundStyle(.red)
    } else if input.isSelectionUnavailable {
      Label("Selected microphone unavailable", systemImage: "exclamationmark.triangle")
        .font(.caption).foregroundStyle(.red)
    }
  }
}

#if os(iOS)
  struct InputDeviceMenu: View {
    @ObservedObject var input: RecordingInput
    let enabled: Bool

    var body: some View {
      Menu {
        if input.fixtureURL != nil {
          Text("Synthetic transcript · microphone off")
        } else if input.configurationError != nil {
          Text("Transcript source unavailable")
        } else {
          Section("Microphone") {
            deviceButton("System Default", uid: "")
            ForEach(input.devices, id: \.uid) { device in
              deviceButton(device.name, uid: device.uid)
            }
            if input.isSelectionUnavailable {
              Label("Selected microphone unavailable", systemImage: "exclamationmark.triangle")
            }
          }
          Divider()
          Button("Refresh", systemImage: "arrow.clockwise", action: input.refresh)
            .menuActionDismissBehavior(.disabled)
            .disabled(!enabled)
          if !enabled { Text("Stop recording to change microphone") }
        }
      } label: {
        Image(systemName: "line.3.horizontal")
          .font(.title3)
          .frame(width: 44, height: 44)
          .contentShape(Rectangle())
      }
      .accessibilityLabel("Microphone menu")
      .accessibilityHint("Choose a microphone or refresh available inputs")
    }

    private func deviceButton(_ name: String, uid: String) -> some View {
      Button {
        input.selectedUID = uid
      } label: {
        if input.selectedUID == uid {
          Label(name, systemImage: "checkmark")
        } else {
          Text(name)
        }
      }
      .disabled(!enabled)
    }
  }
#endif
