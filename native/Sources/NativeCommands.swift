import Foundation
import SKGoNative

@MainActor final class GeneratedCommands: RecordingCommands {
  private let api: NativeAPI
  var selectedProject = ""
  private(set) var recordingProject = ""
  private var activeSession = ""
  init(baseURL: URL) {
    let configuration = URLSessionConfiguration.ephemeral
    configuration.timeoutIntervalForRequest = 2
    configuration.timeoutIntervalForResource = 3
    api = NativeAPI(
      core: RemoteClient(
        origin: baseURL.absoluteString, transport: URLSessionTransport(configuration: configuration)
      ))
  }
  private func validate(_ ack: Ack, sessionID: String, sequence: Int? = nil, closed: Bool) throws {
    guard ack.sessionID == sessionID, sequence == nil || ack.sequence == Int64(sequence!),
      ack.closed == closed
    else { throw RecorderError.message("Recording session acknowledgement did not match.") }
  }
  func open(sessionID: String, source: String) async throws {
    if recordingProject.isEmpty || sessionID != activeSession {
      recordingProject = selectedProject
      activeSession = sessionID
    }
    guard !recordingProject.isEmpty else {
      throw RecorderError.message("Select a project before recording.")
    }
    try validate(
      try await api.openSession(
        OpenRequest(project: recordingProject, sessionID: sessionID, source: source)),
      sessionID: sessionID, sequence: 0, closed: false)
  }
  func activate(sessionID: String) async throws {
    try validate(
      try await api.activateSession(
        SessionRequest(project: recordingProject, sessionID: sessionID)), sessionID: sessionID,
      sequence: 0, closed: false)
  }
  func accept(
    sessionID: String, sequence: Int, finalized: [TranscriptChunk], provisional: [TranscriptChunk]
  ) async throws {
    func convert(_ chunks: [TranscriptChunk]) -> [Chunk] {
      chunks.map { Chunk(startMs: Int64($0.startMs), endMs: Int64($0.endMs), text: $0.text) }
    }
    try validate(
      try await api.acceptTranscript(
        Update(
          project: recordingProject, sessionID: sessionID, sequence: Int64(sequence),
          finalized: convert(finalized), provisional: convert(provisional))), sessionID: sessionID,
      sequence: sequence, closed: false)
  }
  func close(sessionID: String, sequence: Int) async throws {
    try validate(
      try await api.closeSession(
        CloseRequest(project: recordingProject, sessionID: sessionID, sequence: Int64(sequence))),
      sessionID: sessionID, sequence: sequence, closed: true)
  }
  func heartbeat(sessionID: String, state: String) async throws {
    guard !sessionID.isEmpty, !recordingProject.isEmpty, ["recording", "paused"].contains(state)
    else { return }
    _ = try await api.touchSession(
      StateRequest(project: recordingProject, sessionID: sessionID, status: state))
  }
  func abort(sessionID: String) async throws -> Int {
    let ack = try await api.abortSession(
      SessionRequest(project: recordingProject, sessionID: sessionID))
    try validate(ack, sessionID: sessionID, closed: true)
    return Int(ack.sequence)
  }
}
extension RecorderController {
  convenience init(baseURL: URL, input: RecordingInput) {
    self.init(
      commands: GeneratedCommands(baseURL: baseURL),
      makeSource: {
        if let error = input.configurationError { throw RecorderError.message(error) }
        if let fixture = input.fixtureURL { return SyntheticSource(fixtureURL: fixture) }
        return MicrophoneSource(selection: input.selection)
      }, sourceName: input.fixtureURL == nil ? "microphone" : "synthetic")
  }
}
