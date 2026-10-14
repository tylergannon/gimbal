import Foundation
import ScriptedTranscription

@MainActor private final class ScriptedCommands: RecordingCommands {
    struct Update {
        let sequence: Int
        let finalized: [TranscriptChunk]
        let provisional: [TranscriptChunk]
    }
    var onSnapshot: ((String, Int, Bool) -> Void)?
    private(set) var updates: [Update] = []
    private(set) var finals: [TranscriptChunk] = []
    private(set) var tail: [TranscriptChunk] = []
    private(set) var aborts = 0
    private var sessionID = ""
    private var sequence = 0
    private var closed = false
    var text: String { (finals + tail).map(\.text).joined() }
    func open(sessionID: String, source: String, requirements: Bool) async throws {
        precondition(source == "synthetic")
        self.sessionID = sessionID
    }
    func activate(sessionID: String) async throws {
        precondition(sessionID == self.sessionID)
        onSnapshot?(sessionID, 0, false)
    }
    func accept(sessionID: String, sequence: Int, finalized: [TranscriptChunk], provisional: [TranscriptChunk]) async throws {
        precondition(sessionID == self.sessionID && !closed && sequence == self.sequence + 1)
        self.sequence = sequence
        updates.append(Update(sequence: sequence, finalized: finalized, provisional: provisional))
        finals += finalized
        tail = provisional
        onSnapshot?(sessionID, sequence, false)
    }
    func close(sessionID: String, sequence: Int) async throws {
        precondition(sessionID == self.sessionID && sequence == self.sequence)
        closed = true
        onSnapshot?(sessionID, sequence, true)
    }
    func abort(sessionID: String) async throws -> Int {
        precondition(sessionID == self.sessionID)
        closed = true; aborts += 1
        onSnapshot?(sessionID, sequence, true)
        return sequence
    }
}

@main struct ScriptedControllerTests {
    @MainActor private static func wait(_ explanation: String, until condition: () -> Bool) async throws {
        let deadline = ContinuousClock.now.advanced(by: .seconds(2))
        while !condition() {
            guard ContinuousClock.now < deadline else { fatalError("Timed out: \(explanation)") }
            try await Task.sleep(for: .milliseconds(1))
        }
    }
    @MainActor private static func controller(source: SyntheticSource, commands: ScriptedCommands) -> RecorderController {
        let controller = RecorderController(commands: commands, makeSource: { source }, sourceName: "synthetic")
        commands.onSnapshot = { [weak controller] id, sequence, closed in
            controller?.snapshotApplied(sessionID: id, sequence: sequence, closed: closed)
        }
        controller.displayAvailability(true, error: nil)
        return controller
    }
    @MainActor static func main() async throws {
        var now = ContinuousClock.now
        let source = SyntheticSource(events: [
            .init(atMs: 100, startMs: 0, endMs: 100, text: "This was", isFinal: false),
            .init(atMs: 200, startMs: 200, endMs: 300, text: " a test", isFinal: false),
            .init(atMs: 300, startMs: 0, endMs: 150, text: "This is", isFinal: false),
            .init(atMs: 400, startMs: 0, endMs: 150, text: "This is", isFinal: true),
            .init(atMs: 500, startMs: 200, endMs: 350, text: " synthetic text.", isFinal: false),
            .init(atMs: 600, startMs: 200, endMs: 350, text: " synthetic text.", isFinal: true),
            .init(atMs: 700, startMs: 400, endMs: 500, text: " Before pause.", isFinal: false),
            .init(atMs: 800, startMs: 400, endMs: 550, text: " INVALID pause revision", isFinal: false),
            .init(atMs: 900, startMs: 600, endMs: 700, text: " After resume.", isFinal: true)
        ])
        source.now = { now }
        let commands = ScriptedCommands(), recorder = controller(source: source, commands: commands)
        recorder.record()
        try await wait("scripted Record") { recorder.canPause }
        let expectedTexts = ["This was", "This was a test", "This is a test", "This is a test", "This is synthetic text.", "This is synthetic text."]
        for index in expectedTexts.indices {
            now = now.advanced(by: .milliseconds(100))
            try await wait("scripted update \(index + 1)") { commands.updates.count == index + 1 }
            precondition(commands.text == expectedTexts[index])
        }
        precondition(commands.updates[1].provisional.map(\.text) == ["This was", " a test"])
        precondition(commands.updates[2].provisional.map(\.text) == ["This is", " a test"])
        precondition(commands.updates[3].finalized.map(\.text) == ["This is"])
        precondition(commands.updates[3].provisional.map(\.text) == [" a test"])
        precondition(commands.updates[5].provisional.isEmpty)
        precondition(commands.finals.map(\.text) == ["This is", " synthetic text."])
        now = now.advanced(by: .milliseconds(100))
        try await wait("pre-pause provisional") { commands.updates.count == 7 }
        recorder.pause()
        try await wait("Pause drain") { recorder.canResume }
        precondition(commands.finals.last?.text == " Before pause." && commands.tail.isEmpty)
        let pausedUpdates = commands.updates.count
        now = now.advanced(by: .seconds(100))
        recorder.resume()
        try await wait("Resume") { recorder.canPause }
        now = now.advanced(by: .milliseconds(200))
        try await wait("post-resume final") { commands.updates.count == pausedUpdates + 1 }
        precondition(commands.text == "This is synthetic text. Before pause. After resume.")
        recorder.stop()
        try await wait("scripted Stop") { recorder.canRecord && recorder.status.state == "stopped" }
        precondition(commands.aborts == 0 && commands.updates.map(\.sequence) == Array(1...commands.updates.count))

        var invalidNow = ContinuousClock.now
        let invalidSource = SyntheticSource(events: [
            .init(atMs: 100, startMs: 0, endMs: 100, text: "Accepted final.", isFinal: true),
            .init(atMs: 200, startMs: 0, endMs: 150, text: "Invalid late revision.", isFinal: false)
        ])
        invalidSource.now = { invalidNow }
        let invalidCommands = ScriptedCommands(), invalidRecorder = controller(source: invalidSource, commands: invalidCommands)
        invalidRecorder.record()
        try await wait("invalid fixture Record") { invalidRecorder.canPause }
        invalidNow = invalidNow.advanced(by: .milliseconds(100))
        try await wait("ordinary final delivered") { invalidCommands.updates.count == 1 }
        invalidNow = invalidNow.advanced(by: .milliseconds(100))
        try await wait("invalid late result rejected") { invalidRecorder.status.state == "error" && invalidRecorder.canRecord }
        precondition(invalidRecorder.status.error?.contains("invalid audio range") == true)
        precondition(invalidRecorder.status.error?.contains("incomplete") == true)
        precondition(invalidCommands.updates.count == 1 && invalidCommands.text == "Accepted final." && invalidCommands.aborts == 1)
        print("PASS scripted source through controller/normalizer/sender preserves multiple provisional revisions and selective finals, Pause drains/resumes without rewriting finals, ordinary late revision fails visibly without transport mutation")
    }
}
