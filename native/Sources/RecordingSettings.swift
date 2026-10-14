import SwiftUI

#if os(macOS)
  struct RecordingSettings: View {
    @ObservedObject var input: RecordingInput
    let controller: RecorderController?

    var body: some View {
      if let controller {
        RecordingSettingsContent(input: input, controller: controller)
      } else {
        RecordingSettingsForm(input: input, canChange: true)
      }
    }
  }

  private struct RecordingSettingsContent: View {
    @ObservedObject var input: RecordingInput
    @ObservedObject var controller: RecorderController
    var body: some View { RecordingSettingsForm(input: input, canChange: controller.canRecord) }
  }

  private struct RecordingSettingsForm: View {
    @ObservedObject var input: RecordingInput
    let canChange: Bool
    @AppStorage(RecordingSwitchAction.preferenceKey) private var switchAction = RecordingSwitchAction.ask.rawValue
    var body: some View {
      Form {
        Section("Project switching") {
          Picker("While recording", selection: $switchAction) {
            ForEach(RecordingSwitchAction.allCases, id: \.rawValue) { action in
              Text(action.title).tag(action.rawValue)
            }
          }.disabled(!canChange)
        }
        Section("Audio input") {
          if input.fixtureURL != nil || input.configurationError != nil {
            RecordingInputNotice(input: input)
          } else {
            Picker("Microphone", selection: $input.selectedUID) {
              Text("System Default").tag("")
              ForEach(input.devices, id: \.uid) { device in
                Text(device.name).tag(device.uid)
              }
              if input.isSelectionUnavailable {
                Text("Selected microphone unavailable").tag(input.selectedUID)
              }
            }
            .disabled(!canChange)
            .accessibilityHint(
              "System Default follows the Mac's input device. A specific microphone stays selected for the session."
            )

            Button("Refresh Microphones", systemImage: "arrow.clockwise", action: input.refresh)
              .disabled(!canChange)

            Text(
              canChange
                ? "System Default follows your Mac. Choose a specific microphone to keep this session on that device."
                : "Stop recording before changing the microphone."
            )
            .font(.callout).foregroundStyle(.secondary)

            RecordingInputNotice(input: input)
          }
        }
      }
      .formStyle(.grouped)
      .frame(width: 440, height: 360)
    }
  }
#endif
