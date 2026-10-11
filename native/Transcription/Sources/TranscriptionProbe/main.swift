import Foundation
import AppleTranscription
import TranscriptSource

@main struct TranscriptionProbe {
    @MainActor static func main() async {
        let arguments = Array(CommandLine.arguments.dropFirst())
        if arguments == ["--list-inputs"] {
            do { for device in try InputDeviceInventory.inputs() { print("\(device.uid)\t\(device.name)") } }
            catch { fputs("\(error.localizedDescription)\n", stderr); exit(1) }
            return
        }
        guard arguments.count == 4, arguments[0] == "--input", arguments[2] == "--duration",
              let seconds = Int(arguments[3]), (1...120).contains(seconds) else {
            fputs("Usage: transcription-probe --list-inputs | --input default|DEVICE_UID --duration SECONDS (1...120)\n", stderr)
            exit(2)
        }
        let selection: InputDeviceSelection = arguments[1] == "default" ? .systemDefault : .device(uid: arguments[1])
        let source = MicrophoneSource(selection: selection)
        var emittedCount = 0
        source.onEvent = { event in
            emittedCount += 1
            let data = try! JSONEncoder().encode(event.chunk)
            print("\(event.isFinal ? "final" : "partial") \(String(decoding: data, as: UTF8.self))")
        }
        var failure: Error?
        source.onFailure = { failure = $0; source.stopAdmission() }
        // Deadline bounds preparation, capture and drain, including asset download.
        let deadline = Task { @MainActor in
            try? await Task.sleep(for: .seconds(seconds + 30))
            guard !Task.isCancelled else { return }
            source.cancel()
            fputs("Probe deadline expired\n", stderr)
            exit(1)
        }
        defer { deadline.cancel(); source.cancel() }
        do {
            try await source.prepare(); try await source.start()
            try await Task.sleep(for: .seconds(seconds))
            source.stopAdmission(); try await source.drain()
            if let failure { throw failure }
            print("Probe ended; emitted \(emittedCount) events.")
        } catch { fputs("\(error.localizedDescription)\n", stderr); exit(1) }
    }
}
