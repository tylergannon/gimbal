import Combine
import Foundation

#if os(iOS)
  import UIKit
#endif

struct NativeStatus: Codable {
  var sessionID = ""
  var sequence = 0
  var pending = false
  var state = "idle"
  var source = "microphone"
  var error: String?
}
@MainActor protocol RecordingCommands {
  func open(sessionID: String, source: String, requirements: Bool) async throws
  func activate(sessionID: String) async throws
  func accept(
    sessionID: String, sequence: Int, finalized: [TranscriptChunk], provisional: [TranscriptChunk])
    async throws
  func close(sessionID: String, sequence: Int) async throws
  func abort(sessionID: String) async throws -> Int
}
struct RecorderTiming {
  var retry: Duration = .seconds(8)
  var retryDelay: Duration = .milliseconds(250)
  var source: Duration = .seconds(20)
  var preparation: Duration = .seconds(180)
  var display: Duration = .seconds(8)
  var poll: Duration = .milliseconds(50)
}

@MainActor final class RecorderController: ObservableObject {
  @Published private(set) var status = NativeStatus()
  @Published private(set) var audioLevel: Float = 0
  @Published private(set) var displayAvailable = false
  @Published private(set) var externalSessionActive = false
  var onStatus: ((NativeStatus) -> Void)?
  private let commands: any RecordingCommands
  private let makeSource: () throws -> any TranscriptSource
  private let timing: RecorderTiming
  private var source: (any TranscriptSource)?
  private var sender: TranscriptSender?
  private var normalizer = TranscriptNormalizer()
  private var appliedSequence = -1
  private var externalSessionID: String?
  private var externalStopID: String?
  private var externalSequence = 0
  private var closedExternalID: String?
  private var pendingSessionID: String?
  private var pendingAppliedSequence = -1
  private var operation: Task<Void, Never>?
  private var operationID: UUID?
  private var cleanup: Task<Void, Never>?
  private var generation = UUID()
  private var stopRequested = false
  private var sessionOpen = false
  private var deliveryResolved = true
  private var incompleteReason: String?
  private var displayError: String?
  private var staging = false
  private var stagedFinals: [TranscriptChunk] = []
  private var stagedTail: [TranscriptChunk] = []
  private var startupFailure: Error?
  private var sourceGate: CompletionGate?
  private var appBackgrounded = false
  private var resumeDeferred = false
  private(set) var recordingDuration = RecordingDuration()
  private var pendingRollback: (status: NativeStatus, applied: Int, reason: String?)?

  init(
    commands: any RecordingCommands, makeSource: @escaping () throws -> any TranscriptSource,
    sourceName: String = "microphone", timing: RecorderTiming = RecorderTiming()
  ) {
    self.commands = commands
    self.makeSource = makeSource
    self.timing = timing
    status.source = sourceName
  }
  var canRecord: Bool {
    !appBackgrounded && externalSessionID == nil && !externalSessionActive && displayAvailable
      && !sessionOpen && deliveryResolved && operation == nil && cleanup == nil
      && ["idle", "stopped", "error"].contains(status.state)
  }
  var canPause: Bool { status.state == "recording" && operation == nil }
  var canResume: Bool {
    !appBackgrounded && displayAvailable && deliveryResolved && status.state == "paused"
      && operation == nil
  }
  var canStop: Bool {
    externalSessionID != nil || (sessionOpen && status.state != "idle" && status.state != "stopped")
  }
  private func publish(_ state: String) {
    recordingDuration.setRecording(state == "recording")
    if state != "recording" { audioLevel = 0 }
    status.state = state
    status.error = incompleteReason ?? displayError
    onStatus?(status)
  }
  func record(requirements: Bool = false) {
    guard canRecord else { return }
    let previous = status
    let previousApplied = appliedSequence
    incompleteReason = nil
    startupFailure = nil
    pendingRollback = nil
    let operationID = UUID()
    self.operationID = operationID
    let token = UUID()
    generation = token
    stopRequested = false
    let newSource: any TranscriptSource
    do { newSource = try makeSource() } catch {
      self.operationID = nil
      incompleteReason = error.localizedDescription
      publish("error")
      return
    }
    source = newSource
    staging = true
    stagedFinals = []
    stagedTail = []
    normalizer = TranscriptNormalizer()
    newSource.onFailure = { [weak self] error in self?.fail(error, token: token) }
    newSource.onEvent = { [weak self] event in self?.consume(event, token: token) }
    newSource.onLevel = { [weak self] level in
      guard let self, self.generation == token, self.status.state == "recording" else { return }
      let normalized: Float = level.isFinite ? min(1, max(0, level)) : 0
      if self.audioLevel != normalized { self.audioLevel = normalized }
    }
    operation = Task {
      let id = UUID().uuidString
      var opened = false
      var activated = false
      do {
        try ensureCurrent(token)
        try await bounded(timeout: timing.preparation) { try await newSource.prepare() }
        NSLog("Transcription readiness: source preparation completed; opening session")
        try ensureCurrent(token)
        pendingSessionID = id
        pendingAppliedSequence = -1
        // Open is staged: it owns the server session without replacing
        // the prior displayed transcript until capture has started.
        opened = true
        try await retry {
          try await self.commands.open(
            sessionID: id, source: status.source, requirements: requirements)
        }
        NSLog("Transcription readiness: session opened; starting source")
        sessionOpen = true
        try ensureCurrent(token)
        try await bounded {
          guard !self.stopRequested else { throw CancellationError() }
          try await newSource.start()
        }
        try ensureCurrent(token)
        activated = true
        NSLog("Transcription readiness: source started; activating session")
        try await retry { try await self.commands.activate(sessionID: id) }
        try ensureCurrent(token)
        recordingDuration = RecordingDuration()
        status.sessionID = id
        status.sequence = 0
        status.pending = false
        appliedSequence = pendingAppliedSequence
        pendingSessionID = nil
        let commands = self.commands
        let sender = TranscriptSender { sequence, finals, tail in
          try await commands.accept(
            sessionID: id, sequence: sequence, finalized: finals, provisional: tail)
        }
        self.sender = sender
        sender.onDelivery = { [weak self] sequence, pending in
          guard let self, self.generation == token else { return }
          self.status.sequence = sequence
          self.status.pending = pending
          self.onStatus?(self.status)
        }
        sender.onFailure = { [weak controller = self] error in controller?.fail(error, token: token)
        }
        staging = false
        if !stagedFinals.isEmpty || !stagedTail.isEmpty {
          try sender.enqueue(finalized: stagedFinals, provisional: stagedTail)
        }
        stagedFinals = []
        stagedTail = []
        if !stopRequested { publish("recording") }
      } catch {
        NSLog("Transcription readiness: recording setup failed: %@", error.localizedDescription)
        newSource.cancel()
        sender?.cancel()
        generation = UUID()
        let reason = startupFailure ?? error
        let canceledPreparation =
          stopRequested && !activated && startupFailure == nil && error is CancellationError
        incompleteReason =
          activated
          ? "\(reason.localizedDescription) The transcript may be incomplete."
          : reason.localizedDescription
        if opened {
          do {
            let accepted = try await retry { try await self.commands.abort(sessionID: id) }
            sessionOpen = false
            if activated {
              status.sessionID = id
              status.sequence = accepted
              appliedSequence = max(appliedSequence, pendingAppliedSequence)
              deliveryResolved = accepted == 0 || appliedSequence >= accepted
            }
          } catch {
            sessionOpen = true
            deliveryResolved = false
            if !activated {
              pendingRollback = (
                previous, previousApplied, canceledPreparation ? nil : reason.localizedDescription
              )
            }
            status.sessionID = id
            status.sequence = 0
            incompleteReason =
              "\(reason.localizedDescription) Session cleanup is unconfirmed. Use Stop to retry, or relaunch."
          }
        }
        if !activated && !sessionOpen {
          status = previous
          appliedSequence = previousApplied
          deliveryResolved = true
        }
        staging = false
        stagedFinals = []
        stagedTail = []
        pendingSessionID = nil
        if canceledPreparation && !sessionOpen {
          incompleteReason = nil
          publish("stopped")
        } else {
          publish("error")
        }
      }
      finishOperation(operationID)
    }
    publish("preparing")
  }
  private func finishOperation(_ id: UUID) {
    guard operationID == id else { return }
    operation = nil
    operationID = nil
  }
  private func ensureCurrent(_ token: UUID) throws {
    if token != generation && stopRequested && startupFailure == nil { throw CancellationError() }
    guard token == generation, startupFailure == nil else {
      throw startupFailure ?? RecorderError.message("Recording ended during setup.")
    }
  }
  private func consume(_ event: TranscriptEvent, token: UUID) {
    guard token == generation else { return }
    do {
      let normalized = try normalizer.consume(event)
      if staging {
        stagedFinals += normalized.finalized
        stagedTail = normalized.provisional
        guard try JSONEncoder().encode(stagedFinals + stagedTail).count <= 256 * 1024 else {
          throw RecorderError.message("Transcript setup exceeded its memory limit.")
        }
      } else {
        try sender?.enqueue(finalized: normalized.finalized, provisional: normalized.provisional)
      }
    } catch { fail(error, token: token) }
  }
  func pause() {
    guard canPause else { return }
    let token = generation
    let operationID = UUID()
    let finalization = backgroundFinalization(token: token)
    source?.stopAdmission()
    self.operationID = operationID
    operation = Task {
      defer { finalization.end() }
      do {
        try await bounded { try await self.source?.drain() }
        try await sender?.flush()
        try ensureCurrent(token)
        status.sequence = sender?.sequence ?? 0
        onStatus?(status)
        try await confirmDisplay()
        if !stopRequested { publish("paused") }
      } catch { fail(error, token: token) }
      finishOperation(operationID)
    }
    publish("pausing")
  }
  func resume() {
    guard canResume else { return }
    resumeDeferred = false
    let token = generation
    let operationID = UUID()
    self.operationID = operationID
    operation = Task {
      do {
        try await bounded {
          guard !self.stopRequested && !self.resumeDeferred && !self.appBackgrounded else {
            throw CancellationError()
          }
          try await self.source?.start()
        }
        try ensureCurrent(token)
        if resumeDeferred { throw CancellationError() }
        if !stopRequested { publish("recording") }
      } catch {
        if resumeDeferred && !stopRequested && token == generation {
          // Backgrounding during Resume cancels this segment's startup,
          // not the existing paused session. Seal/drain any admitted audio.
          let finalization = backgroundFinalization(token: token)
          defer { finalization.end() }
          do {
            try await bounded { try await self.source?.drain() }
            try await sender?.flush()
            try ensureCurrent(token)
            status.sequence = sender?.sequence ?? 0
            try await confirmDisplay()
            if !stopRequested { publish("paused") }
          } catch { fail(error, token: token) }
        } else if !(stopRequested && error is CancellationError) {
          fail(error, token: token)
        }
      }
      finishOperation(operationID)
    }
    publish("resuming")
  }
  func stop() {
    source?.stopAdmission()
    // Lifecycle Stop can arrive before a session exists (permissions/assets).
    // Keep the product button disabled, but latch cancellation and resolve the
    // source wait immediately. An uncertain Open is still aborted below.
    if status.state == "preparing" && !sessionOpen {
      stopRequested = true
      generation = UUID()
      source?.cancel()
      sourceGate?.finish(.failure(CancellationError()))
      publish("stopping")
      return
    }
    if !sessionOpen, let id = externalSessionID {
      stopExternal(id)
      return
    }
    guard canStop else { return }
    if incompleteReason != nil {
      beginCleanup()
      return
    }
    if status.state == "stopping" { return }
    let preceding = operation
    let token = generation
    let operationID = UUID()
    let finalization = backgroundFinalization(token: token)
    self.operationID = operationID
    stopRequested = true
    operation = Task {
      defer { finalization.end() }
      if let preceding { await preceding.value }
      // Startup may already have aborted and settled. Do not overwrite
      // that result or drain a canceled source with the old generation.
      guard sessionOpen, token == generation else {
        finishOperation(operationID)
        return
      }
      if incompleteReason != nil {
        beginCleanup()
        finishOperation(operationID)
        return
      }
      publish("stopping")
      do {
        try await bounded { try await self.source?.drain() }
        try await sender?.flush()
        try ensureCurrent(token)
        status.sequence = sender?.sequence ?? 0
        let id = status.sessionID
        let sequence = status.sequence
        try await retry { try await self.commands.close(sessionID: id, sequence: sequence) }
        sessionOpen = false
        onStatus?(status)
        try await confirmDisplay()
        source?.cancel()
        source = nil
        publish("stopped")
      } catch { fail(error, token: token) }
      finishOperation(operationID)
    }
    publish("stopping")
  }
  private func stopExternal(_ id: String) {
    guard operation == nil else { return }
    let operationID = UUID()
    self.operationID = operationID
    externalStopID = id
    status.sessionID = id
    status.sequence = externalSequence
    appliedSequence = externalSequence
    incompleteReason = nil
    operation = Task {
      do {
        status.sequence = try await retry { try await self.commands.abort(sessionID: id) }
        let deadline = ContinuousClock.now.advanced(by: timing.display)
        while closedExternalID != id || appliedSequence < status.sequence {
          guard ContinuousClock.now < deadline else {
            throw RecorderError.message(
              "External session stopped, but final transcript display is unconfirmed. Use Stop to retry, or relaunch."
            )
          }
          try await Task.sleep(for: timing.poll)
        }
        externalSessionID = nil
        externalSessionActive = false
        deliveryResolved = true
        publish("stopped")
      } catch {
        incompleteReason =
          "\(error.localizedDescription) External session cleanup is unconfirmed. Use Stop to retry, or relaunch."
        publish("error")
      }
      externalStopID = nil
      finishOperation(operationID)
    }
    publish("stopping")
  }
  func snapshotApplied(sessionID: String, sequence: Int, closed: Bool? = nil) {
    if let closed {
      if sessionID == externalSessionID {
        externalSequence = max(externalSequence, sequence)
        if closed {
          closedExternalID = sessionID
          externalSessionActive = false
          if externalStopID != sessionID { externalSessionID = nil }
        }
      } else if !closed && !sessionID.isEmpty && sessionID != status.sessionID
        && sessionID != pendingSessionID
      {
        externalSessionID = sessionID
        externalSequence = sequence
        closedExternalID = nil
        externalSessionActive = true
      }
    }
    if sessionID == pendingSessionID {
      pendingAppliedSequence = max(pendingAppliedSequence, sequence)
    }
    guard sessionID == status.sessionID else { return }
    appliedSequence = max(appliedSequence, sequence)
    if appliedSequence >= status.sequence { deliveryResolved = true }
  }
  func displayAvailability(_ available: Bool, error: String?) {
    let nextError = available ? nil : error ?? "Transcript display is reconnecting."
    guard displayAvailable != available || displayError != nextError else { return }
    displayAvailable = available
    displayError = available ? nil : error ?? "Transcript display is reconnecting."
    status.error = incompleteReason ?? displayError
    onStatus?(status)
  }
  private func confirmDisplay() async throws {
    let deadline = ContinuousClock.now.advanced(by: timing.display)
    while appliedSequence < status.sequence {
      // A suspended web view cannot acknowledge display. Keep accepted
      // text in the native server and reconcile when it returns.
      if appBackgrounded {
        deliveryResolved = false
        return
      }
      guard ContinuousClock.now < deadline else {
        deliveryResolved = false
        throw RecorderError.message(
          "Final transcript display is unconfirmed. Relaunch if the display cannot recover.")
      }
      try await Task.sleep(for: timing.poll)
    }
    deliveryResolved = true
  }
  private func fail(_ error: Error, token: UUID) {
    guard token == generation else { return }
    generation = UUID()
    source?.cancel()
    sender?.cancel()
    if staging {
      startupFailure = error
      return
    }
    incompleteReason = "\(error.localizedDescription) The transcript may be incomplete."
    deliveryResolved = false
    publish("error")
    beginCleanup()
  }
  private func beginCleanup() {
    guard cleanup == nil, sessionOpen else { return }
    let id = status.sessionID
    let finalization = backgroundFinalization(token: generation)
    cleanup = Task {
      defer { finalization.end() }
      var settledState = "error"
      do {
        let sequence = try await retry { try await self.commands.abort(sessionID: id) }
        sessionOpen = false
        if let rollback = pendingRollback {
          status = rollback.status
          appliedSequence = rollback.applied
          pendingRollback = nil
          deliveryResolved = true
          incompleteReason = rollback.reason
          settledState = rollback.reason == nil ? "stopped" : "error"
        } else {
          status.sequence = sequence
          status.pending = false
          onStatus?(status)
          try await confirmDisplay()
        }
      } catch {
        incompleteReason =
          "\(incompleteReason ?? "Transcript may be incomplete.") Session or display cleanup is unconfirmed. Use Stop to retry, or relaunch."
      }
      publish(settledState)
      cleanup = nil
    }
  }
  func setAppBackgrounded(_ value: Bool) {
    appBackgrounded = value
    // Permissions/assets/startup still require the visible app. An already
    // active recording owns its audio session and continues unchanged.
    if value && status.state == "preparing" { stop() }
    if value && status.state == "resuming" && !resumeDeferred {
      resumeDeferred = true
      source?.stopAdmission()
      sourceGate?.finish(.failure(CancellationError()))
    }
  }
  func shutdown() {
    audioLevel = 0
    sourceGate?.finish(.failure(CancellationError()))
    operation?.cancel()
    cleanup?.cancel()
    generation = UUID()
    source?.cancel()
    sender?.cancel()
  }
  private func backgroundFinalization(token: UUID) -> RecordingFinalization {
    RecordingFinalization { [weak self] in
      self?.fail(
        RecorderError.message("Background time expired while delivering the final transcript."),
        token: token)
    }
  }
  private func retry<T>(_ action: () async throws -> T) async throws -> T {
    let deadline = ContinuousClock.now.advanced(by: timing.retry)
    while true {
      try Task.checkCancellation()
      do { return try await action() } catch {
        guard ContinuousClock.now < deadline else { throw error }
        try await Task.sleep(for: timing.retryDelay)
      }
    }
  }
  private func bounded(
    timeout: Duration? = nil, _ action: @escaping @MainActor () async throws -> Void
  ) async throws {
    let gate = CompletionGate()
    sourceGate = gate
    defer { if sourceGate === gate { sourceGate = nil } }
    try await withCheckedThrowingContinuation { continuation in
      gate.continuation = continuation
      gate.action = Task {
        do {
          try await action()
          gate.finish(.success(()))
        } catch { gate.finish(.failure(error)) }
      }
      gate.timeout = Task {
        try? await Task.sleep(for: timeout ?? self.timing.source)
        if !Task.isCancelled {
          gate.finish(
            .failure(RecorderError.message("Speech preparation or finalization timed out.")))
        }
      }
    }
  }
}
// Keep the finite audio drain AND its local delivery alive after capture stops.
// Each operation owns its grant, so joined Pause/Stop tasks cannot end each other's.
@MainActor private final class RecordingFinalization {
  #if os(iOS)
    private var identifier: UIBackgroundTaskIdentifier = .invalid
  #endif
  init(onExpiration: @escaping @MainActor () -> Void) {
    #if os(iOS)
      identifier = UIApplication.shared.beginBackgroundTask(withName: "Finish transcript delivery")
      { [weak self] in
        guard let self else { return }
        self.end()
        onExpiration()
      }
    #endif
  }
  func end() {
    #if os(iOS)
      let task = identifier
      identifier = .invalid
      if task != .invalid { UIApplication.shared.endBackgroundTask(task) }
    #endif
  }
}
@MainActor private final class CompletionGate {
  var continuation: CheckedContinuation<Void, Error>?
  var action: Task<Void, Never>?
  var timeout: Task<Void, Never>?
  func finish(_ result: Result<Void, Error>) {
    guard let continuation else { return }
    self.continuation = nil
    timeout?.cancel()
    action?.cancel()
    continuation.resume(with: result)
  }
}

// A monotonic session stopwatch, independent of UI redraws and wall-clock changes.
struct RecordingDuration {
  private var accumulated: TimeInterval = 0
  private var started: TimeInterval?
  private static let origin = ContinuousClock.now
  private static var currentTime: TimeInterval {
    let duration = origin.duration(to: .now).components
    return Double(duration.seconds) + Double(duration.attoseconds) / 1e18
  }

  mutating func setRecording(_ recording: Bool, now suppliedTime: TimeInterval? = nil) {
    let now = suppliedTime ?? Self.currentTime
    if recording {
      if started == nil { started = now }
    } else if let started {
      accumulated += max(0, now - started)
      self.started = nil
    }
  }
  func elapsed(now suppliedTime: TimeInterval? = nil) -> TimeInterval {
    let now = suppliedTime ?? Self.currentTime
    return accumulated + (started.map { max(0, now - $0) } ?? 0)
  }
}
