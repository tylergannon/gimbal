import Foundation
import TranscriptSource
import AVFoundation
import Speech
import CoreMedia
#if os(iOS)
import UIKit
#endif

// Converter priming/status handling adapted from Apple SwiftTranscriptionSampleApp
// Helpers/BufferConversion.swift (2025). Format-change replacement adapted from
// rryam/AuralKit BufferConverter.swift at 8e8f93cb9afdd4e11432bc14667eb6d020a9e96a.
// Copyright 2025 Apple Inc.; Copyright 2025 Rudrank Riyam.
// Notices: native/Resources/Licenses/. Exact source details: ephemeral/worklog/native-swift.md.

// The tap and Pause/Stop share this lock. Once close returns no buffer can enter
// the analyzer stream. Buffers are converted synchronously while tap memory lives.
final class AudioIntake: @unchecked Sendable {
    private let lock = NSLock()
    private var accepting = true
    private var frames: Int64 = 0
    private var capturedSeconds = 0.0
    private var converter: AVAudioConverter
    private let outputFormat: AVAudioFormat
    private let continuation: AsyncStream<AnalyzerInput>.Continuation
    private var conversionFailure: Error?
    private let onFailure: @Sendable (Error) -> Void
    private let onLevel: @Sendable (Float) -> Void
    private var nextLevelTime = 0.0
    init(input: AVAudioFormat, output: AVAudioFormat, continuation: AsyncStream<AnalyzerInput>.Continuation, onFailure: @escaping @Sendable (Error) -> Void = { _ in }, onLevel: @escaping @Sendable (Float) -> Void = { _ in }) throws {
        guard let converter = AVAudioConverter(from: input, to: output) else { throw RecorderError.message("Microphone audio could not be converted for speech analysis.") }
        converter.primeMethod = .none
        self.converter = converter; outputFormat = output; self.continuation = continuation; self.onFailure = onFailure
        self.onLevel = onLevel
    }
    func receive(_ buffer: AVAudioPCMBuffer) {
        lock.lock(); defer { lock.unlock() }
        guard accepting else { return }
        do {
            guard buffer.format.sampleRate > 0, buffer.format.channelCount > 0 else { throw RecorderError.message("Microphone audio format is unavailable.") }
            // A default device's live format can change. Finish its old converter
            // before replacing it so accepted frames are retained on the timeline.
            if converter.inputFormat != buffer.format {
                try flushConverter()
                guard let replacement = AVAudioConverter(from: buffer.format, to: outputFormat) else { throw RecorderError.message("The changed microphone format could not be converted.") }
                replacement.primeMethod = .none
                converter = replacement
            }
            capturedSeconds += Double(buffer.frameLength) / buffer.format.sampleRate
            let capacity = AVAudioFrameCount(ceil(Double(buffer.frameLength) * outputFormat.sampleRate / buffer.format.sampleRate) + 32)
            guard let output = AVAudioPCMBuffer(pcmFormat: outputFormat, frameCapacity: capacity) else { throw RecorderError.message("Microphone conversion buffer could not be allocated.") }
            var consumed = false
            var error: NSError?
            let status = converter.convert(to: output, error: &error) { _, status in
                if consumed { status.pointee = .noDataNow; return nil }
                consumed = true; status.pointee = .haveData; return buffer
            }
            guard status != .error, error == nil else { throw error ?? RecorderError.message("Microphone audio conversion failed.") }
            try emit(output)
            if capturedSeconds + 0.000_001 >= nextLevelTime {
                nextLevelTime = capturedSeconds + 0.1
                onLevel(AudioInputLevel.normalized(buffer))
            }
        } catch {
            conversionFailure = error; accepting = false; continuation.finish(); onFailure(error)
        }
    }
    private func emit(_ output: AVAudioPCMBuffer) throws {
        // Resampler flush may append filter padding. Only captured audio advances
        // the session timeline; discard samples beyond admitted input duration.
        let remaining = max(0, Int64((capturedSeconds * outputFormat.sampleRate).rounded()) - frames)
        output.frameLength = min(output.frameLength, AVAudioFrameCount(remaining))
        guard output.frameLength > 0 else { return }
        let time = CMTime(value: frames, timescale: CMTimeScale(outputFormat.sampleRate))
        frames += Int64(output.frameLength)
        switch continuation.yield(AnalyzerInput(buffer: output, bufferStartTime: time)) {
        case .dropped: throw RecorderError.message("Speech analysis could not keep up with microphone audio.")
        case .terminated: throw RecorderError.message("Speech analysis stopped accepting microphone audio.")
        case .enqueued: break
        @unknown default: throw RecorderError.message("Speech audio delivery returned an unknown state.")
        }
    }
    private func flushConverter() throws {
        guard let output = AVAudioPCMBuffer(pcmFormat: outputFormat, frameCapacity: 4096) else { throw RecorderError.message("Audio finalization buffer could not be allocated.") }
        var error: NSError?
        let status = converter.convert(to: output, error: &error) { _, status in status.pointee = .endOfStream; return nil }
        guard status != .error, error == nil else { throw error ?? RecorderError.message("Microphone audio finalization failed.") }
        try emit(output)
    }
    func close() throws -> Int {
        lock.lock(); defer { lock.unlock() }
        if accepting {
            accepting = false
            do { try flushConverter() } catch { conversionFailure = error }
        }
        continuation.finish()
        if let conversionFailure { throw conversionFailure }
        return Int(Double(frames) / outputFormat.sampleRate * 1000)
    }
}

// Configuration notifications arrive off actor; every callback is tied to one
// live capture segment and invalidated before expected Pause/Stop teardown.
@MainActor final class AudioConfigurationWatch {
    private var observers: [NSObjectProtocol] = []
    private var token: UUID?
    public func start(engine: AVAudioEngine, onChange: @escaping @MainActor () -> Void) {
        invalidate()
        let token = UUID(); self.token = token
        func observe(_ name: Notification.Name, object: Any?) {
            observers.append(NotificationCenter.default.addObserver(forName: name, object: object, queue: nil) { @Sendable [weak self] _ in
                Task { @MainActor in
                    guard let self, self.token == token else { return }
                    self.invalidate(); onChange()
                }
            })
        }
        observe(.AVAudioEngineConfigurationChange, object: engine)
        #if os(iOS)
        observe(AVAudioSession.interruptionNotification, object: AVAudioSession.sharedInstance())
        observe(AVAudioSession.routeChangeNotification, object: AVAudioSession.sharedInstance())
        #endif
    }
    func invalidate() {
        token = nil
        for observer in observers { NotificationCenter.default.removeObserver(observer) }
        observers = []
    }
}

@MainActor public final class MicrophoneSource: TranscriptSource {
    public var onEvent: ((TranscriptEvent) -> Void)?
    public var onFailure: ((Error) -> Void)?
    public var onLevel: ((Float) -> Void)?
    public let selection: InputDeviceSelection
    public init(selection: InputDeviceSelection = .systemDefault) { self.selection = selection }
    private var engine: AVAudioEngine?
    private var analyzer: SpeechAnalyzer?
    private var transcriber: SpeechTranscriber?
    private var resultTask: Task<Void, Error>?
    private var intake: AudioIntake?
    private var timeline = CapturedAudioTimeline()
    private var lastFinalEnding: Character?
    private var inputEpoch = UUID()
    private var sealedDuration: Int?
    private var sealedFailure: Error?
    private var tapInstalled = false
    private let configurationWatch = AudioConfigurationWatch()
    private var assetRequest: AssetInstallationRequest?
    #if os(iOS)
    private var drainBackgroundTask: UIBackgroundTaskIdentifier = .invalid
    private var drainBackgroundEpoch: UUID?
    private var drainBackgroundExpired = false
    private func beginBackgroundDrain() {
        guard drainBackgroundTask == .invalid, !drainBackgroundExpired,
              engine?.isRunning == true || analyzer != nil else { return }
        let epoch = UUID(); drainBackgroundEpoch = epoch
        drainBackgroundTask = UIApplication.shared.beginBackgroundTask(withName: "Finish admitted speech") { [weak self] in
            guard let self, self.drainBackgroundEpoch == epoch else { return }
            self.drainBackgroundExpired = true
            self.endBackgroundDrain()
            let failure = RecorderError.message("Background time expired while finishing speech. The transcript may be incomplete.")
            self.sealedFailure = failure
            let reportFailure = self.onFailure
            self.cancel()
            reportFailure?(failure)
        }
    }
    private func endBackgroundDrain() {
        let task = drainBackgroundTask
        drainBackgroundTask = .invalid; drainBackgroundEpoch = nil
        if task != .invalid { UIApplication.shared.endBackgroundTask(task) }
    }
    #endif
    public func prepare() async throws {
        NSLog("Transcription readiness: microphone authorization status=%ld", AVCaptureDevice.authorizationStatus(for: .audio).rawValue)
        guard await AVCaptureDevice.requestAccess(for: .audio) else { throw RecorderError.message("Microphone permission was denied. Allow access in System Settings to record.") }
        try Task.checkCancellation()
        NSLog("Transcription readiness: microphone authorized; speech authorization status=%ld", SFSpeechRecognizer.authorizationStatus().rawValue)
        let speechAllowed = await PermissionCallback.result { @Sendable completion in
            SFSpeechRecognizer.requestAuthorization { @Sendable status in completion(status == .authorized) }
        }
        try Task.checkCancellation()
        guard speechAllowed else { throw RecorderError.message("Speech recognition permission was denied.") }
        NSLog("Transcription readiness: speech authorized; checking on-device English locale")
        guard SpeechTranscriber.isAvailable, let locale = await SpeechTranscriber.supportedLocale(equivalentTo: Locale(identifier: "en-US")) else { throw RecorderError.message("On-device English speech transcription is unavailable on this device.") }
        try Task.checkCancellation()
        let module = SpeechTranscriber(locale: locale, transcriptionOptions: [], reportingOptions: [.volatileResults], attributeOptions: [.audioTimeRange])
        NSLog("Transcription readiness: locale supported; checking speech assets")
        if let request = try await AssetInventory.assetInstallationRequest(supporting: [module]) {
            NSLog("Transcription readiness: downloading speech assets")
            assetRequest = request
            defer { assetRequest = nil }
            try Task.checkCancellation()
            try await request.downloadAndInstall()
            NSLog("Transcription readiness: speech asset download completed")
        }
        try Task.checkCancellation()
        guard await AssetInventory.status(forModules: [module]) == .installed else { throw RecorderError.message("On-device speech assets are unavailable.") }
        NSLog("Transcription readiness: on-device speech assets installed")
        transcriber = module
    }
    private func checkStart(_ epoch: UUID) throws {
        try Task.checkCancellation()
        guard inputEpoch == epoch else { throw CancellationError() }
    }
    public func start() async throws {
        try Task.checkCancellation()
        NSLog("Transcription readiness: starting audio segment")
        let epoch = UUID(); inputEpoch = epoch
        sealedDuration = nil; sealedFailure = nil
        #if os(iOS)
        drainBackgroundExpired = false
        let session = AVAudioSession.sharedInstance()
        try InputDeviceInventory.configureRecordingSession(session)
        try session.setActive(true)
        try InputDeviceInventory.apply(selection, session: session)
        #endif
        // A fresh module/analyzer per segment produces local audio-relative times.
        guard let locale = await SpeechTranscriber.supportedLocale(equivalentTo: Locale(identifier: "en-US")) else { throw RecorderError.message("Speech locale is unavailable.") }
        try checkStart(epoch)
        let module = SpeechTranscriber(locale: locale, transcriptionOptions: [], reportingOptions: [.volatileResults], attributeOptions: [.audioTimeRange])
        transcriber = module
        let engine = AVAudioEngine()
        #if os(macOS)
        try InputDeviceInventory.apply(selection, engine: engine)
        NSLog("Transcription readiness: input device selected")
        #endif
        let input = engine.inputNode.outputFormat(forBus: 0)
        guard input.sampleRate > 0, input.channelCount > 0, let format = await SpeechAnalyzer.bestAvailableAudioFormat(compatibleWith: [module], considering: input) else { throw RecorderError.message("The default microphone is unavailable.") }
        try checkStart(epoch)
        NSLog("Transcription readiness: analyzer format available; preparing analyzer")
        let analyzer = SpeechAnalyzer(modules: [module])
        do {
            try await analyzer.prepareToAnalyze(in: format)
            NSLog("Transcription readiness: analyzer prepared")
            try checkStart(epoch)
        } catch {
            Task { await analyzer.cancelAndFinishNow() }
            throw error
        }
        let (stream, continuation) = AsyncStream<AnalyzerInput>.makeStream(bufferingPolicy: .bufferingNewest(128))
        let intake = try AudioIntake(input: input, output: format, continuation: continuation, onFailure: { [weak self] error in
            Task { @MainActor in
                guard let self, self.inputEpoch == epoch else { return }
                self.onLevel?(0)
                self.onFailure?(error)
            }
        }, onLevel: { [weak self] level in
            Task { @MainActor in
                guard let self, self.inputEpoch == epoch else { return }
                self.onLevel?(level)
            }
        })
        self.intake = intake; self.engine = engine; self.analyzer = analyzer
        let segmentTimeline = timeline
        let precedingFinalEnding = lastFinalEnding
        resultTask = Task { [weak self] in
            do {
                var spacing = SpeechSegmentSpacing(precedingFinalEnding: precedingFinalEnding)
                for try await result in module.results {
                    try Task.checkCancellation()
                    let start = CMTimeGetSeconds(result.range.start)
                    let end = CMTimeGetSeconds(CMTimeRangeGetEnd(result.range))
                    guard start.isFinite, end.isFinite else { throw RecorderError.message("Speech returned an invalid timestamp.") }
                    let text = spacing.text(String(result.text.characters), isFinal: result.isFinal)
                    if result.isFinal, let ending = text.last { self?.lastFinalEnding = ending }
                    self?.onEvent?(TranscriptEvent(chunk: try segmentTimeline.chunk(startSeconds: start, endSeconds: end, text: text), isFinal: result.isFinal))
                }
            } catch { if !Task.isCancelled { self?.onFailure?(error) }; throw error }
        }
        try await analyzer.start(inputSequence: stream)
        NSLog("Transcription readiness: analyzer started")
        try checkStart(epoch)
        engine.inputNode.installTap(onBus: 0, bufferSize: 1024, format: input) { @Sendable buffer, _ in intake.receive(buffer) }
        tapInstalled = true
        engine.prepare()
        try checkStart(epoch)
        do { try engine.start() } catch { cancel(); throw error }
        NSLog("Transcription readiness: audio engine started")
        configurationWatch.start(engine: engine) { [weak self] in
            self?.onFailure?(RecorderError.message("The audio device configuration changed or recording was interrupted. Recording has ended; start a new recording when audio is available."))
        }
        guard engine.isRunning else { cancel(); throw RecorderError.message("The microphone stopped during setup.") }
    }
    public func stopAdmission() {
        #if os(iOS)
        // Audio background execution ends when the engine stops. Retain only
        // the finite grant needed to finalize audio already admitted.
        beginBackgroundDrain()
        #endif
        inputEpoch = UUID()
        onLevel?(0)
        configurationWatch.invalidate()
        if sealedDuration == nil && sealedFailure == nil {
            do { sealedDuration = try intake?.close() ?? 0 } catch { sealedFailure = error }
        }
        if tapInstalled { engine?.inputNode.removeTap(onBus: 0); tapInstalled = false }
        engine?.stop()
    }
    public func drain() async throws {
        stopAdmission()
        #if os(iOS)
        defer { endBackgroundDrain() }
        #endif
        if let sealedFailure { throw sealedFailure }
        let duration = sealedDuration ?? 0
        engine = nil
        if let analyzer { try await analyzer.finalizeAndFinishThroughEndOfInput() }
        if let resultTask { try await resultTask.value }
        if let sealedFailure { throw sealedFailure }
        try timeline.finishSegment(capturedMs: duration)
        self.resultTask = nil; self.analyzer = nil; intake = nil
        sealedDuration = nil; sealedFailure = nil
        #if os(iOS)
        try? AVAudioSession.sharedInstance().setActive(false)
        #endif
    }
    public func cancel() {
        stopAdmission()
        #if os(iOS)
        defer { endBackgroundDrain() }
        #endif
        configurationWatch.invalidate()
        assetRequest?.progress.cancel(); assetRequest = nil
        _ = try? intake?.close(); intake = nil
        engine?.stop(); engine = nil
        resultTask?.cancel(); resultTask = nil
        if let analyzer { Task { await analyzer.cancelAndFinishNow() } }
        analyzer = nil; onEvent = nil; onFailure = nil; onLevel = nil
    }
}
