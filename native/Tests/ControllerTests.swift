import Foundation

@MainActor final class FakeSource: TranscriptSource {
    var onLevel: ((Float) -> Void)?
    var onEvent: ((TranscriptEvent) -> Void)?
    var onFailure: ((Error) -> Void)?
    var prepareDelay: Duration = .zero
    var ignorePrepareCancellation = false
    private(set) var starts = 0
    var startFails = false
    var startDelay: Duration = .zero
    var ignoreStartCancellation = false
    private var inputEpoch = UUID()
    private(set) var admissions = 0
    var startEvent: TranscriptEvent?
    var afterStart: (() -> Void)?
    private(set) var admitting = false
    var drainDelay: Duration = .zero
    var drainEvent: TranscriptEvent?
    private(set) var cancels = 0
    private(set) var drains = 0
    private var concurrentDrains = 0
    private(set) var maxConcurrentDrains = 0
    func prepare() async throws { if ignorePrepareCancellation { try? await Task.sleep(for: prepareDelay) } else { try await Task.sleep(for: prepareDelay) } }
    func start() async throws {
        starts += 1
        let epoch = UUID(); inputEpoch = epoch
        if ignoreStartCancellation { try? await Task.sleep(for: startDelay) } else { try await Task.sleep(for: startDelay) }
        if startFails { throw RecorderError.message("Input could not start") }
        guard epoch == inputEpoch else { throw CancellationError() }
        admissions += 1; admitting = true
        if let startEvent { onEvent?(startEvent) }
        afterStart?()
    }
    func stopAdmission() { inputEpoch = UUID(); admitting = false }

    func drain() async throws {
        drains += 1; concurrentDrains += 1
        maxConcurrentDrains = max(maxConcurrentDrains, concurrentDrains)
        defer { concurrentDrains -= 1 }
        try await Task.sleep(for: drainDelay)
        if let drainEvent { self.drainEvent = nil; onEvent?(drainEvent) }
    }
    func cancel() { stopAdmission(); cancels += 1; onEvent = nil; onFailure = nil }
}
@MainActor final class FakeCommands: RecordingCommands {
    var hasSelectedProject = true
    struct Session { var sequence = 0; var activated = false; var closed = false }
    var sessions: [String: Session] = [:]
    var activeID = ""
    var displayText = "previous words"
    var shouldNotify = true
    var abortFails = false
    var loseAbortResponse = false
    var openDelay: Duration = .zero
    private(set) var openCalls = 0
    var closeFails = false
    var closeDuration = -1
    var closeDelay: Duration = .zero
    var openUnreachable = false
    var loseOpenResponse = false
    var canceledIDs: [String: Int] = [:]
    var activationDelay: Duration = .zero
    var activationFails = false
    var loseActivationResponse = false
    var onSnapshot: ((String, Int) -> Void)?
    private(set) var closes = 0
    private(set) var aborts = 0
    private(set) var requirements: [Bool] = []
    func open(sessionID: String, source: String, requirements: Bool) async throws {
        openCalls += 1; self.requirements.append(requirements); try await Task.sleep(for: openDelay)
        if openUnreachable { throw RecorderError.message("Open unreachable") }
        guard canceledIDs[sessionID] == nil else { throw RecorderError.message("Canceled session") }
        if sessions[sessionID] != nil { if loseOpenResponse { throw RecorderError.message("Lost open response") }; return }
        guard sessions[activeID]?.closed != false else { throw RecorderError.message("Previous session open") }
        activeID = sessionID; sessions[sessionID] = Session()
        if loseOpenResponse { throw RecorderError.message("Lost open response") }
    }
    func activate(sessionID: String) async throws {
        if activationFails { try await Task.sleep(for: activationDelay); throw RecorderError.message("Activation unavailable") }
        sessions[sessionID]?.activated = true; displayText = ""
        if shouldNotify { onSnapshot?(sessionID, 0) }
        try await Task.sleep(for: activationDelay) // Receipt arrives before ack.
        if loseActivationResponse { throw RecorderError.message("Lost activation response") }
    }
    func accept(sessionID: String, sequence: Int, finalized: [TranscriptChunk], provisional: [TranscriptChunk]) async throws {
        guard var state = sessions[sessionID], !state.closed, sequence == state.sequence + 1 else { throw RecorderError.message("Invalid update") }
        state.sequence = sequence; sessions[sessionID] = state
        displayText += finalized.map(\.text).joined() + provisional.map(\.text).joined()
        if shouldNotify { onSnapshot?(sessionID, sequence) }
    }
    func close(sessionID: String, sequence: Int, durationMs: Int) async throws {
        try await Task.sleep(for: closeDelay)
        if closeFails { throw RecorderError.message("Save unavailable") }
        closeDuration = durationMs
        guard sessions[sessionID]?.sequence == sequence else { throw RecorderError.message("Close across gap") }
        sessions[sessionID]?.closed = true; closes += 1
        if shouldNotify { onSnapshot?(sessionID, sequence) }
    }
    func abort(sessionID: String) async throws -> Int {
        aborts += 1
        if abortFails { throw RecorderError.message("Cleanup unavailable") }
        if let sequence = canceledIDs[sessionID] { if loseAbortResponse { throw RecorderError.message("Lost abort response") }; return sequence }
        guard let state = sessions[sessionID] else { canceledIDs[sessionID] = 0; return 0 }
        sessions[sessionID]?.closed = true; canceledIDs[sessionID] = state.sequence
        if state.activated && shouldNotify { onSnapshot?(sessionID, state.sequence) }
        if loseAbortResponse { throw RecorderError.message("Lost abort response") }
        return state.sequence
    }
}
@main struct ControllerTests {
    @MainActor static func wait(_ explanation: String, until condition: () -> Bool) async throws {
        let deadline = ContinuousClock.now.advanced(by: .seconds(2))
        while !condition() {
            guard ContinuousClock.now < deadline else { fatalError("Timed out: \(explanation)") }
            try await Task.sleep(for: .milliseconds(1))
        }
    }
    @MainActor static func make(_ source: FakeSource, commands: FakeCommands, timing: RecorderTiming = RecorderTiming(retry: .milliseconds(30), retryDelay: .milliseconds(2), source: .milliseconds(200), preparation: .milliseconds(200), display: .milliseconds(200), poll: .milliseconds(1))) -> RecorderController {
        let controller = RecorderController(commands: commands, makeSource: { source }, timing: timing)
        commands.onSnapshot = { [weak controller] id, sequence in controller?.snapshotApplied(sessionID: id, sequence: sequence) }
        controller.displayAvailability(true, error: nil)
        return controller
    }
    @MainActor static func main() async throws {
        var stopwatch = RecordingDuration()
        stopwatch.setRecording(true, now: 100)
        precondition(stopwatch.elapsed(now: 103) == 3)
        stopwatch.setRecording(false, now: 103)
        precondition(stopwatch.elapsed(now: 200) == 3)
        stopwatch.setRecording(true, now: 200)
        stopwatch.setRecording(true, now: 201) // repeated state notification does not reset.
        precondition(stopwatch.elapsed(now: 205) == 8)
        stopwatch.setRecording(false, now: 205)
        precondition(stopwatch.elapsed(now: 300) == 8)
        stopwatch = RecordingDuration()
        precondition(stopwatch.elapsed(now: 300) == 0)

        let admissionSource = FakeSource(), admissionCommands = FakeCommands()
        let admissionController = make(admissionSource, commands: admissionCommands)
        admissionCommands.hasSelectedProject = false
        admissionController.record()
        admissionController.record(requirements: true)
        precondition(admissionSource.starts == 0 && admissionCommands.openCalls == 0 && admissionCommands.aborts == 0)
        precondition(admissionController.status.error == "Select a project before recording.")
        admissionCommands.hasSelectedProject = true
        admissionController.record()
        try await wait("record after admission without restart") { admissionController.canPause }
        admissionCommands.closeDelay = .milliseconds(30)
        let closingID = admissionController.status.sessionID
        admissionController.projectSwitchPending = true
        precondition(!admissionController.canPause && !admissionController.canRecord)
        try await admissionController.stopAndSave()
        precondition(admissionController.status.state == "stopped" && admissionCommands.sessions[closingID]?.closed == true)
        precondition(admissionCommands.closes == 1 && admissionCommands.closeDuration >= 0)
        admissionController.projectSwitchPending = false
        admissionController.record(requirements: true)
        try await wait("new recording identity") { admissionController.canPause }
        precondition(admissionController.status.sessionID != closingID && admissionController.status.requirements)
        admissionCommands.closeFails = true
        do { try await admissionController.stopAndSave(); fatalError("failed save reported success") }
        catch { precondition(admissionController.status.state == "error") }
        precondition(admissionController.canStop && !admissionController.canRecord && admissionCommands.aborts == 0)
        admissionCommands.closeFails = false
        try await admissionController.stopAndSave()
        precondition(admissionController.status.state == "stopped" && admissionCommands.closes == 2)

        let levelSource = FakeSource(), levelCommands = FakeCommands()
        let levelController = make(levelSource, commands: levelCommands)
        levelController.record()
        try await wait("level session active") { levelController.canPause }
        let oldLevelCallback = levelSource.onLevel
        levelSource.onLevel?(0.7)
        precondition(levelController.audioLevel == 0.7)
        levelSource.onLevel?(.nan)
        precondition(levelController.audioLevel == 0)
        levelSource.onLevel?(2)
        precondition(levelController.audioLevel == 1)
        levelController.pause()
        precondition(levelController.audioLevel == 0)
        oldLevelCallback?(0.9)
        precondition(levelController.audioLevel == 0)
        try await wait("level session paused") { levelController.canResume }
        levelController.resume()
        try await wait("level session resumed") { levelController.canPause }
        levelSource.onLevel?(0.4)
        precondition(levelController.audioLevel == 0.4)
        levelController.stop()
        precondition(levelController.audioLevel == 0)
        try await wait("level session stopped") { levelController.canRecord }
        oldLevelCallback?(0.9)
        precondition(levelController.audioLevel == 0)
        levelController.record()
        try await wait("new level session active") { levelController.canPause }
        oldLevelCallback?(0.9)
        precondition(levelController.audioLevel == 0)
        levelSource.onLevel?(0.3)
        precondition(levelController.audioLevel == 0.3)
        levelController.stop()
        try await wait("new level session stopped") { levelController.canRecord }

        let externalGate = make(FakeSource(), commands: FakeCommands())
        externalGate.snapshotApplied(sessionID: "external", sequence: 1, closed: false)
        precondition(!externalGate.canRecord && externalGate.canStop)
        externalGate.snapshotApplied(sessionID: "external", sequence: 1, closed: true)
        precondition(externalGate.canRecord)

        // An abandoned external caller can be stopped without starting a source.
        let externalSource = FakeSource(), externalCommands = FakeCommands()
        externalCommands.sessions["abandoned"] = FakeCommands.Session(sequence: 3, activated: true)
        externalCommands.activeID = "abandoned"
        let external = make(externalSource, commands: externalCommands)
        external.snapshotApplied(sessionID: "abandoned", sequence: 3, closed: false)
        externalCommands.onSnapshot = { id, sequence in external.snapshotApplied(sessionID: id, sequence: sequence, closed: true) }
        externalCommands.abortFails = true
        external.stop(); external.stop()
        try await wait("external abort failure") { external.status.state == "error" }
        precondition(external.canStop && !external.canRecord && externalSource.starts == 0)
        externalCommands.abortFails = false; externalCommands.shouldNotify = false
        external.stop()
        try await wait("external abort accepted") { externalCommands.sessions["abandoned"]?.closed == true }
        precondition(external.status.state == "stopping" && !external.canRecord)
        external.snapshotApplied(sessionID: "unrelated", sequence: 99, closed: true)
        precondition(external.status.state == "stopping")
        external.snapshotApplied(sessionID: "abandoned", sequence: 3, closed: true)
        try await wait("external closed display confirmed") { external.canRecord && external.status.state == "stopped" }
        precondition(!external.canStop && externalSource.starts == 0)
        externalCommands.shouldNotify = true
        externalCommands.onSnapshot = { id, sequence in external.snapshotApplied(sessionID: id, sequence: sequence) }
        external.record(); try await wait("Record after external Stop") { external.canPause }
        external.stop(); try await wait("native Stop after external") { external.canRecord }

        // A closed receipt during repeated lost abort replies must not permit
        // native capture while a stale external Stop target is retained.
        let lostAbortSource = FakeSource(), lostAbortCommands = FakeCommands()
        lostAbortCommands.sessions["lost-abort"] = FakeCommands.Session(sequence: 2, activated: true)
        lostAbortCommands.activeID = "lost-abort"; lostAbortCommands.loseAbortResponse = true
        let lostAbort = make(lostAbortSource, commands: lostAbortCommands)
        lostAbort.snapshotApplied(sessionID: "lost-abort", sequence: 2, closed: false)
        lostAbortCommands.onSnapshot = { id, sequence in lostAbort.snapshotApplied(sessionID: id, sequence: sequence, closed: true) }
        lostAbort.setAppBackgrounded(true); lostAbort.setAppBackgrounded(false)
        precondition(lostAbortCommands.aborts == 0 && lostAbort.canStop)
        lostAbort.stop()
        try await wait("closed receipt with lost abort response") { lostAbort.status.state == "error" }
        precondition(!lostAbort.canRecord && lostAbort.canStop)
        lostAbort.record(); precondition(lostAbortSource.starts == 0)
        lostAbortCommands.loseAbortResponse = false; lostAbort.stop()
        try await wait("lost abort retry resolves") { lostAbort.canRecord }
        lostAbortCommands.onSnapshot = { id, sequence in lostAbort.snapshotApplied(sessionID: id, sequence: sequence) }
        lostAbort.record(); try await wait("native capture after resolved abort") { lostAbort.canPause }
        lostAbort.stop(); try await wait("native capture stopped after external recovery") { lostAbort.canRecord }
        precondition(lostAbortSource.cancels > 0)

        // A remote session can start and close while native preparation has
        // not reserved the server yet. Its closed receipt must not leave a
        // stale Stop target, and background Stop must cancel native preparation.
        for backgrounded in [false, true] {
            let preparationSource = FakeSource(), preparationCommands = FakeCommands()
            preparationSource.prepareDelay = .milliseconds(80)
            let preparation = make(preparationSource, commands: preparationCommands)
            preparation.record()
            preparation.snapshotApplied(sessionID: "concurrent-external", sequence: 1, closed: false)
            if backgrounded { preparation.setAppBackgrounded(true); preparation.setAppBackgrounded(false) }
            preparation.snapshotApplied(sessionID: "concurrent-external", sequence: 1, closed: true)
            if backgrounded {
                try await wait("background canceled native preparation with external snapshot") { preparation.canRecord }
                precondition(preparationSource.starts == 0 && preparation.status.state == "stopped")
            } else {
                try await wait("native preparation after external close") { preparation.canPause }
                preparation.stop(); try await wait("native stop has no stale external target") { preparation.canRecord }
                precondition(!preparation.canStop && preparation.status.state == "stopped")
            }
        }

        // Backgrounding retains the active source/session while the display is
        // suspended. Accepted words are still available when it reconnects.
        let backgroundSource = FakeSource(), backgroundCommands = FakeCommands()
        let background = make(backgroundSource, commands: backgroundCommands)
        background.record(); try await wait("background initial recording") { background.canPause }
        let backgroundID = background.status.sessionID
        backgroundSource.onEvent?(TranscriptEvent(chunk: .init(startMs: 0, endMs: 100, text: "before "), isFinal: true))
        try await wait("foreground words accepted") { backgroundCommands.displayText == "before " }
        backgroundCommands.shouldNotify = false
        background.setAppBackgrounded(true)
        background.displayAvailability(false, error: "Display suspended")
        backgroundSource.onEvent?(TranscriptEvent(chunk: .init(startMs: 100, endMs: 200, text: "during background "), isFinal: true))
        try await wait("background words accepted") { backgroundCommands.displayText == "before during background " }
        try await Task.sleep(for: .milliseconds(250)) // Beyond the display deadline.
        precondition(background.status.state == "recording" && background.status.sessionID == backgroundID)
        precondition(backgroundSource.starts == 1 && backgroundSource.cancels == 0 && backgroundSource.drains == 0)
        precondition(backgroundCommands.closes == 0 && backgroundCommands.aborts == 0)
        background.setAppBackgrounded(false)
        background.snapshotApplied(sessionID: backgroundID, sequence: backgroundCommands.sessions[backgroundID]!.sequence)
        background.displayAvailability(true, error: nil)
        backgroundCommands.shouldNotify = true
        background.stop(); try await wait("stop after background") { background.canRecord }
        precondition(background.status.sessionID == backgroundID && background.status.error == nil)
        precondition(backgroundCommands.displayText == "before during background ")

        // Pause draining while the app backgrounds cannot fail just because the
        // hidden web view cannot acknowledge its final update.
        let hiddenSource = FakeSource(), hiddenCommands = FakeCommands()
        let hidden = make(hiddenSource, commands: hiddenCommands)
        hidden.record(); try await wait("hidden pause initial recording") { hidden.canPause }
        hiddenCommands.shouldNotify = false
        hiddenSource.drainEvent = TranscriptEvent(chunk: .init(startMs: 0, endMs: 100, text: "drained hidden words"), isFinal: true)
        hidden.pause(); hidden.setAppBackgrounded(true)
        try await wait("hidden pause settled") { hidden.status.state == "paused" }
        precondition(hidden.status.error == nil && hiddenSource.cancels == 0 && !hidden.canResume)
        hidden.setAppBackgrounded(false)
        precondition(!hidden.canResume)
        hidden.snapshotApplied(sessionID: hidden.status.sessionID, sequence: hidden.status.sequence)
        precondition(hidden.canResume && hiddenCommands.displayText == "drained hidden words")
        hiddenCommands.shouldNotify = true
        hidden.stop(); try await wait("hidden pause stopped") { hidden.canRecord }

        // Background during queued or delayed Resume retains the paused session.
        for queuedResume in [true, false] {
            let resumeSource = FakeSource(), resumeCommands = FakeCommands()
            let resumed = make(resumeSource, commands: resumeCommands)
            resumed.record(); try await wait("resume background initial") { resumed.canPause }
            resumeSource.drainEvent = TranscriptEvent(chunk: .init(startMs: 0, endMs: 100, text: "retained paused text"), isFinal: true)
            resumed.pause(); try await wait("resume background paused") { resumed.canResume }
            let retainedID = resumed.status.sessionID
            resumeSource.startDelay = .milliseconds(50); resumeSource.ignoreStartCancellation = true
            resumed.resume()
            if !queuedResume { try await wait("resume startup entered") { resumeSource.starts == 2 } }
            resumed.setAppBackgrounded(true)
            try await wait("background resume returns to paused") { resumed.status.state == "paused" }
            try await Task.sleep(for: .milliseconds(60))
            precondition(resumed.status.sessionID == retainedID && resumed.status.error == nil)
            precondition(resumeCommands.aborts == 0 && resumeCommands.closes == 0 && resumeCommands.displayText == "retained paused text")
            precondition(resumeSource.admissions == 1 && resumeSource.cancels == 0)
            resumed.setAppBackgrounded(false)
            precondition(resumed.canResume)
            resumeSource.startDelay = .zero; resumed.resume()
            try await wait("resume foreground succeeds") { resumed.canPause }
            precondition(resumed.status.sessionID == retainedID && resumeSource.admissions == 2)
            resumed.stop(); try await wait("resume foreground stopped") { resumed.canRecord }
        }

        // A second background transition during the deferred drain must not
        // cancel that drain or abort the existing paused recording.
        let bouncingSource = FakeSource(), bouncingCommands = FakeCommands()
        let bouncing = make(bouncingSource, commands: bouncingCommands)
        bouncing.record(); try await wait("bounce initial") { bouncing.canPause }
        bouncing.pause(); try await wait("bounce paused") { bouncing.canResume }
        let bouncingID = bouncing.status.sessionID
        bouncingSource.startDelay = .milliseconds(50); bouncingSource.ignoreStartCancellation = true
        bouncingSource.drainDelay = .milliseconds(100)
        bouncing.resume(); try await wait("bounce resume entered") { bouncingSource.starts == 2 }
        bouncing.setAppBackgrounded(true)
        try await wait("bounce deferred drain entered") { bouncingSource.drains == 2 }
        bouncing.setAppBackgrounded(false); bouncing.setAppBackgrounded(true)
        try await wait("bounce retains pause") { bouncing.status.state == "paused" }
        precondition(bouncing.status.sessionID == bouncingID && bouncing.status.error == nil && bouncingCommands.aborts == 0)
        bouncing.setAppBackgrounded(false); bouncingSource.drainDelay = .zero
        bouncing.stop(); try await wait("bounce stopped") { bouncing.canRecord }

        // Schedule background on the actor hop between source completion and
        // the Resume continuation. Recording must never describe a sealed source.
        let completionSource = FakeSource(), completionCommands = FakeCommands()
        let completion = make(completionSource, commands: completionCommands)
        completion.record(); try await wait("completion initial") { completion.canPause }
        completion.pause(); try await wait("completion paused") { completion.canResume }
        var completionBackgrounded = false
        completionSource.afterStart = {
            Task { @MainActor in completion.setAppBackgrounded(true); completionBackgrounded = true }
        }
        completion.resume()
        try await wait("completion background settled") { completionBackgrounded && ["paused", "recording"].contains(completion.status.state) }
        precondition(completion.status.error == nil && completionCommands.aborts == 0)
        precondition(completion.status.state != "recording" || completionSource.admitting)
        completion.setAppBackgrounded(false); completionSource.afterStart = nil
        completion.stop(); try await wait("completion stopped") { completion.canRecord }

        let invalidSource = RecorderController(commands: FakeCommands(), makeSource: { throw RecorderError.message("Invalid source") })
        invalidSource.displayAvailability(true, error: nil); invalidSource.record()
        precondition(invalidSource.status.state == "error" && invalidSource.status.error == "Invalid source" && invalidSource.canRecord)

        // iPhone lifecycle calls Stop while permissions/assets are still pending.
        // Even a late source prepare completion cannot start background capture.
        let preparingSource = FakeSource(), preparingCommands = FakeCommands()
        preparingSource.prepareDelay = .milliseconds(100); preparingSource.ignorePrepareCancellation = true
        let preparing = make(preparingSource, commands: preparingCommands)
        preparing.record()
        try await Task.sleep(for: .milliseconds(20))
        precondition(preparing.status.state == "preparing" && !preparing.canStop)
        preparing.stop(); preparing.stop(); preparing.pause()
        try await wait("background preparation stopped") { preparing.status.state == "stopped" && preparing.canRecord }
        try await Task.sleep(for: .milliseconds(120))
        precondition(preparingSource.starts == 0 && preparingSource.cancels > 0 && preparingCommands.sessions.isEmpty && preparingCommands.displayText == "previous words")
        let openingSource = FakeSource(), openingCommands = FakeCommands()
        openingCommands.openDelay = .milliseconds(50)
        let opening = make(openingSource, commands: openingCommands)
        opening.record(); try await wait("Open in flight") { openingCommands.openCalls == 1 }
        precondition(!opening.canStop)
        opening.stop()
        try await wait("background uncertain Open aborted") { opening.status.state == "stopped" && opening.canRecord }
        precondition(openingSource.starts == 0 && openingCommands.aborts == 1 && openingCommands.sessions.values.allSatisfy { $0.closed } && openingCommands.displayText == "previous words")
        let blockedOpenSource = FakeSource(), blockedOpenCommands = FakeCommands()
        blockedOpenCommands.openDelay = .milliseconds(20); blockedOpenCommands.abortFails = true
        let blockedOpen = make(blockedOpenSource, commands: blockedOpenCommands)
        blockedOpen.record(); try await wait("blocked Open in flight") { blockedOpenCommands.openCalls == 1 }
        blockedOpen.stop()
        try await wait("pending abort failed") { blockedOpen.status.state == "error" && blockedOpen.canStop }
        blockedOpenCommands.abortFails = false; blockedOpen.stop()
        try await wait("pending rollback recovered") { blockedOpen.canRecord }
        precondition(blockedOpenSource.starts == 0 && blockedOpenCommands.displayText == "previous words" && blockedOpen.status.state == "stopped" && blockedOpen.status.error == nil)
        // A genuine Open error survives successful rollback; cleanup advice does not.
        let unreachableSource = FakeSource(), unreachableCommands = FakeCommands()
        unreachableCommands.openUnreachable = true; unreachableCommands.abortFails = true
        let unreachable = make(unreachableSource, commands: unreachableCommands)
        unreachable.record(); try await wait("unreachable pending rollback") { unreachable.status.state == "error" && unreachable.canStop }
        unreachableCommands.abortFails = false; unreachable.stop()
        try await wait("unreachable rollback confirmed") { unreachable.canRecord }
        precondition(unreachable.status.state == "error" && unreachable.status.error == "Open unreachable" && unreachableCommands.displayText == "previous words")
        // Stop after Open joins startup, but cannot hide a genuine start failure.
        let startFailureSource = FakeSource(), startFailureCommands = FakeCommands()
        startFailureSource.startFails = true; startFailureSource.startDelay = .milliseconds(10)
        let startFailure = make(startFailureSource, commands: startFailureCommands)
        var failureStates: [String] = []
        startFailure.onStatus = { failureStates.append($0.state) }
        startFailure.record(); try await wait("failing start Open") { startFailure.canStop }
        startFailure.stop(); startFailure.pause()
        try await wait("joined failing start settled") { startFailure.status.state == "error" && startFailure.canRecord }
        try await Task.sleep(for: .milliseconds(30))
        precondition(startFailure.status.state == "error" && startFailure.status.error == "Input could not start" && !failureStates.contains("stopped") && !failureStates.contains("recording") && startFailureSource.drains == 0 && startFailureSource.maxConcurrentDrains == 0 && startFailureCommands.displayText == "previous words")
        for activationAccepted in [false, true] {
            let activationSource = FakeSource(), activationCommands = FakeCommands()
            activationCommands.activationDelay = .milliseconds(10)
            activationCommands.activationFails = !activationAccepted
            activationCommands.loseActivationResponse = activationAccepted
            let activation = make(activationSource, commands: activationCommands)
            activation.record(); try await wait("activation Open") { activationSource.admissions == 1 }
            activation.stop(); activation.pause()
            try await wait("failed activation settled") { activation.status.state == "error" && activation.canRecord }
            precondition(activation.status.error?.contains(activationAccepted ? "Lost activation response" : "Activation unavailable") == true)
            precondition(activation.status.error?.contains("cleanup is unconfirmed") != true && activationSource.drains == 0 && activationCommands.sessions.values.allSatisfy { $0.closed })
            precondition(activationCommands.displayText == (activationAccepted ? "" : "previous words"))
        }
        // Stop during delayed startup owns the task until completion. The prior
        // Record task cannot flash Recording or clear Stop's operation handle.
        let startingSource = FakeSource(), startingCommands = FakeCommands()
        startingSource.startDelay = .milliseconds(50); startingSource.drainDelay = .milliseconds(30)
        let starting = make(startingSource, commands: startingCommands)
        var startingStates: [String] = []
        var pauseEnabledAfterStop = false
        var requestedStop = false
        starting.onStatus = { status in
            startingStates.append(status.state)
            if requestedStop {
                pauseEnabledAfterStop = pauseEnabledAfterStop || starting.canPause
                starting.pause() // Reentrant Pause probe while stopping.
            }
        }
        starting.record(); try await wait("startup open") { starting.canStop }
        requestedStop = true; starting.stop(); starting.stop(); starting.pause()
        try await wait("startup Stop completed") { starting.status.state == "stopped" && starting.canRecord }
        let afterStop = startingStates.drop { $0 != "stopping" }
        precondition(!afterStop.contains("recording") && !afterStop.contains("pausing") && !pauseEnabledAfterStop)
        precondition(startingSource.drains == 0 && startingSource.maxConcurrentDrains == 0 && startingCommands.aborts == 1 && startingSource.admissions == 0)
        // A delayed startup that ignores task cancellation cannot admit audio
        // after Stop, on either the first segment or Resume.
        for resumed in [false, true] {
            let delayedSource = FakeSource(), delayedCommands = FakeCommands()
            let delayed = make(delayedSource, commands: delayedCommands)
            if resumed {
                delayed.record(); try await wait("initial segment") { delayed.canPause }
                delayedSource.drainEvent = TranscriptEvent(chunk: .init(startMs: 0, endMs: 100, text: "before Stop "), isFinal: true)
                delayed.pause(); try await wait("initial pause") { delayed.canResume }
            }
            let admittedBefore = delayedSource.admissions
            delayedSource.startDelay = .milliseconds(50)
            delayedSource.ignoreStartCancellation = true
            delayedSource.startEvent = TranscriptEvent(chunk: .init(startMs: 100, endMs: 200, text: "AFTER Stop"), isFinal: true)
            if resumed { delayed.resume() } else { delayed.record() }
            try await wait("delayed startup entered") { delayedSource.starts == (resumed ? 2 : 1) }
            delayed.stop()
            precondition(!delayed.canPause && !delayed.canResume && !delayed.canRecord)
            try await wait("sealed delayed startup") { delayed.canRecord }
            try await Task.sleep(for: .milliseconds(60))
            precondition(delayed.status.state == "stopped" && delayed.status.error == nil)
            precondition(delayedSource.admissions == admittedBefore && delayedSource.maxConcurrentDrains <= 1)
            precondition(delayedCommands.displayText == (resumed ? "before Stop " : "previous words"))
        }
        // Stop can also arrive before the queued Resume task enters start().
        let queuedSource = FakeSource(), queuedCommands = FakeCommands()
        let queued = make(queuedSource, commands: queuedCommands)
        queued.record(); try await wait("queued initial segment") { queued.canPause }
        queued.pause(); try await wait("queued pause") { queued.canResume }
        queuedSource.startEvent = TranscriptEvent(chunk: .init(startMs: 0, endMs: 100, text: "queued AFTER Stop"), isFinal: true)
        queued.resume(); queued.stop()
        try await wait("queued resume stopped") { queued.canRecord }
        precondition(queuedSource.admissions == 1 && queued.status.state == "stopped" && queued.status.error == nil && queuedCommands.displayText.isEmpty)
        // Open may never arrive, or may arrive with every response lost. Abort
        // resolves both cases without clearing the previous displayed text.
        for responseLost in [false, true] {
            let openSource = FakeSource(), openCommands = FakeCommands()
            openCommands.openUnreachable = !responseLost; openCommands.loseOpenResponse = responseLost
            let openController = make(openSource, commands: openCommands)
            openController.record()
            try await wait("uncertain open cleaned") { openController.status.state == "error" && openController.canRecord }
            precondition(openCommands.displayText == "previous words" && !openController.canStop)
            let canceled = openCommands.canceledIDs.keys.first!
            openCommands.openUnreachable = false; openCommands.loseOpenResponse = false
            do { try await openCommands.open(sessionID: canceled, source: "synthetic", requirements: false); fatalError("Delayed canceled Open accepted") } catch {}
            openController.record(); try await wait("Record after open recovery") { openController.canPause }
            openController.stop(); try await wait("Recovered session stopped") { openController.canRecord }
        }
        // Setup failure closes the staged session, preserving old visible text.
        let bad = FakeSource(); bad.startFails = true
        bad.startEvent = TranscriptEvent(chunk: .init(startMs: 0, endMs: 1, text: "must never publish"), isFinal: true)
        let badCommands = FakeCommands(); let failedSetup = make(bad, commands: badCommands)
        failedSetup.record()
        try await wait("failed setup cleaned") { failedSetup.status.state == "error" && failedSetup.canRecord }
        precondition(badCommands.displayText == "previous words" && badCommands.aborts == 1 && bad.cancels > 0)
        // Receipt before activate acknowledgement is retained, including seq0.
        let earlySource = FakeSource(), earlyCommands = FakeCommands()
        earlyCommands.activationDelay = .milliseconds(20)
        let early = make(earlySource, commands: earlyCommands)
        early.record(); try await wait("recording") { early.canPause }
        early.pause(); try await wait("empty pause") { early.status.state == "paused" }
        early.stop(); try await wait("empty stop") { early.status.state == "stopped" && early.canRecord }
        early.record(requirements: true); try await wait("requirements recording") { early.canPause }
        precondition(earlyCommands.requirements == [false, true])
        early.stop(); try await wait("requirements stop") { early.status.state == "stopped" && early.canRecord }
        // A drained final cannot settle Pause or Stop ahead of displayed text.
        let source = FakeSource(), commands = FakeCommands()
        commands.shouldNotify = false
        let controller = make(source, commands: commands)
        controller.record(); try await wait("recording for drain") { controller.canPause }
        let id = controller.status.sessionID
        source.drainEvent = TranscriptEvent(chunk: .init(startMs: 0, endMs: 500, text: "final words "), isFinal: true)
        source.drainDelay = .milliseconds(15)
        controller.pause(); controller.stop(); controller.stop()
        try await wait("drained final accepted") { controller.status.sequence == 1 }
        precondition(controller.status.state == "stopping" && !controller.canResume && !controller.canRecord)
        controller.snapshotApplied(sessionID: "stale", sequence: 999)
        try await Task.sleep(for: .milliseconds(10)); precondition(controller.status.state == "stopping")
        controller.snapshotApplied(sessionID: id, sequence: 1)
        try await wait("joined stop") { controller.status.state == "stopped" && controller.canRecord }
        precondition(commands.closes == 1 && commands.displayText == "final words ")
        // Source failure remains visibly incomplete even after Stop/abort succeeds.
        let failedSource = FakeSource(), failureCommands = FakeCommands()
        failureCommands.abortFails = true
        let failure = make(failedSource, commands: failureCommands)
        failure.record(); try await wait("failure recording") { failure.canPause }
        let staleCallback = failedSource.onEvent
        failedSource.onFailure?(RecorderError.message("Analyzer failed"))
        try await wait("failed cleanup") { failure.status.state == "error" && failure.status.error?.contains("cleanup is unconfirmed") == true }
        precondition(!failure.canRecord && failure.canStop && failedSource.cancels > 0)
        failureCommands.abortFails = false
        failure.stop(); failure.stop()
        try await wait("failure recovery") { failure.canRecord }
        precondition(failure.status.state == "error" && failure.status.error?.contains("incomplete") == true)
        let abortCount = failureCommands.aborts
        staleCallback?(TranscriptEvent(chunk: .init(startMs: 0, endMs: 1, text: "stale"), isFinal: true))
        precondition(failureCommands.displayText.isEmpty && failureCommands.aborts == abortCount)
        failure.record(); try await wait("new recording after failure") { failure.canPause }
        let newID = failure.status.sessionID
        staleCallback?(TranscriptEvent(chunk: .init(startMs: 0, endMs: 1, text: "old generation"), isFinal: true))
        failure.snapshotApplied(sessionID: id, sequence: 999)
        try await Task.sleep(for: .milliseconds(5))
        precondition(failure.status.sessionID == newID && failureCommands.sessions[newID]?.sequence == 0 && failureCommands.displayText.isEmpty)
        failure.stop(); try await wait("new recording stopped") { failure.canRecord }
        // Cancel a drain that exceeds its finite budget; never claim Paused.
        let hungSource = FakeSource(), hungCommands = FakeCommands()
        let short = RecorderTiming(retry: .milliseconds(30), retryDelay: .milliseconds(2), source: .milliseconds(15), preparation: .milliseconds(100), display: .milliseconds(100), poll: .milliseconds(1))
        let hung = make(hungSource, commands: hungCommands, timing: short)
        hung.record(); try await wait("hung recording") { hung.canPause }
        hungSource.drainDelay = .seconds(1)
        hung.pause(); try await wait("drain timeout") { hung.status.state == "error" && hung.canRecord }
        precondition(hungSource.cancels > 0 && hung.status.error?.contains("timed out") == true)
        print("PASS Resume background reentry/actor-completion race retains consistent source and session, background retains source/session and delivers hidden words, hidden Pause defers display confirmation, delayed/queued Record and Resume cannot admit after Stop even ignoring cancellation, background Stop during delayed preparation prevents capture/preserves prior display, Stop during startup preserves operation ownership/no concurrent drains, never-arrived/lost-response open recovery, controller staged setup rollback, early sequence0 receipt, pending-final display gate, Stop joins Pause, stale receipt/callback rejection, failure cleanup retry preserves incomplete error, bounded drain cancellation")
    }
}
