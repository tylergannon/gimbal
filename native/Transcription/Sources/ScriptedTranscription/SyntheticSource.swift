import Foundation
import TranscriptSource

@MainActor public final class SyntheticSource: TranscriptSource {
    public struct Fixture: Codable, Sendable {
        public let atMs: Int; public let startMs: Int; public let endMs: Int; public let text: String; public let isFinal: Bool
        public init(atMs: Int, startMs: Int, endMs: Int, text: String, isFinal: Bool) { self.atMs = atMs; self.startMs = startMs; self.endMs = endMs; self.text = text; self.isFinal = isFinal }
    }
    public var onEvent: ((TranscriptEvent) -> Void)?
    public var onFailure: ((Error) -> Void)?
    public var onLevel: ((Float) -> Void)?
    private var events: [Fixture] = []
    private var index = 0
    private var elapsedMs = 0
    private var started: ContinuousClock.Instant?
    private var timer: Task<Void, Never>?
    private var playbackEpoch = UUID()
    private var lastLevelTick: Int?
    private var provisional: [TranscriptChunk] = []
    public var now: () -> ContinuousClock.Instant = { .now }
    private let fixtureURL: URL?
    public init(events: [Fixture]) { self.events = events; fixtureURL = nil }
    public init(fixtureURL: URL) { self.fixtureURL = fixtureURL }
    public func prepare() async throws {
        if !events.isEmpty { return }
        guard let url = fixtureURL else { throw RecorderError.message("The synthetic fixture is missing.") }
        events = try JSONDecoder().decode([Fixture].self, from: Data(contentsOf: url))
    }
    public func start() async throws {
        timer?.cancel()
        let epoch = UUID(); playbackEpoch = epoch
        started = now()
        lastLevelTick = nil
        playbackTick()
        timer = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .milliseconds(40))
                guard !Task.isCancelled, let self, self.playbackEpoch == epoch else { return }
                self.playbackTick()
            }
        }
    }
    // Deterministic simulated amplitude; never opens an audio device. It shares
    // the fixture's captured-time clock, including its pause/resume boundary.
    func playbackTick() {
        guard let started else { return }
        let duration = started.duration(to: now()).components
        let current = elapsedMs + Int(duration.seconds * 1000 + duration.attoseconds / 1_000_000_000_000_000)
        let tick = current / 100
        if lastLevelTick != tick {
            lastLevelTick = tick
            let levels: [Float] = [0, 0, 0.15, 0.4, 0.7, 0.9, 0.65, 0.3, 0.1, 0]
            onLevel?(levels[max(0, tick) % levels.count])
        }
        emit(through: current)
    }
    private func emit(through milliseconds: Int) {
        while index < events.count && events[index].atMs <= milliseconds {
            let item = events[index]; index += 1
            let chunk = TranscriptChunk(startMs: item.startMs, endMs: item.endMs, text: item.text)
            // Pausing finalizes the already emitted partial; skip fixture revisions
            // belonging to that finalized range when playback resumes.
            // Ordinary final results do not suppress invalid later fixture events:
            // they must reach the same normalizer that checks Apple's results.
            if let drainedBoundary, chunk.startMs < drainedBoundary { continue }
            provisional.removeAll { $0.endMs > chunk.startMs && $0.startMs < chunk.endMs || $0.startMs == chunk.startMs }
            if item.isFinal {
                finalizedBoundary = chunk.endMs
                provisional.removeAll { $0.startMs < finalizedBoundary }
            } else {
                provisional.append(chunk)
                provisional.sort { $0.startMs < $1.startMs }
            }
            onEvent?(TranscriptEvent(chunk: chunk, isFinal: item.isFinal))
        }
    }
    private var finalizedBoundary = 0
    private var drainedBoundary: Int?
    public func stopAdmission() {
        playbackEpoch = UUID()
        if let started {
            let duration = started.duration(to: now()).components
            elapsedMs += Int(duration.seconds * 1000 + duration.attoseconds / 1_000_000_000_000_000)
            emit(through: elapsedMs)
        }
        started = nil; timer?.cancel(); timer = nil
        onLevel?(0)
    }
    public func drain() async throws {
        stopAdmission()
        let admitted = provisional
        provisional = []
        for chunk in admitted {
            finalizedBoundary = chunk.endMs
            drainedBoundary = chunk.endMs
            onEvent?(TranscriptEvent(chunk: chunk, isFinal: true))
        }
    }
    public func cancel() { stopAdmission(); started = nil; timer?.cancel(); timer = nil; onEvent = nil; onFailure = nil; onLevel = nil }
}
