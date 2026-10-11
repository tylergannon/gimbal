import Foundation
public struct TranscriptChunk: Codable, Equatable, Sendable {
    public init(startMs: Int, endMs: Int, text: String) { self.startMs = startMs; self.endMs = endMs; self.text = text }
    public var startMs: Int
    public var endMs: Int
    public var text: String
}
public struct TranscriptEvent: Sendable {
    public init(chunk: TranscriptChunk, isFinal: Bool) { self.chunk = chunk; self.isFinal = isFinal }
    public var chunk: TranscriptChunk
    public var isFinal: Bool
}
public enum RecorderError: LocalizedError {
    case message(String)
    public var errorDescription: String? { if case let .message(value) = self { return value }; return nil }
}
@MainActor public protocol TranscriptSource: AnyObject {
    var onEvent: ((TranscriptEvent) -> Void)? { get set }
    var onFailure: ((Error) -> Void)? { get set }
    // Input amplitude, normalized to 0...1. Delivery is on the main actor;
    // stopAdmission/cancel clear it synchronously before result draining.
    var onLevel: ((Float) -> Void)? { get set }
    func prepare() async throws
    func start() async throws
    // Synchronous capture boundary: freeze audio time and invalidate pending
    // starts, while retaining already captured results for drain().
    func stopAdmission()
    func drain() async throws
    func cancel()
}

public extension TranscriptSource {
    // Sources without audio (including existing test doubles) need no meter.
    var onLevel: ((Float) -> Void)? { get { nil } set {} }
}

public struct CapturedAudioTimeline {
    public init() {}
    public private(set) var offsetMs = 0
    public mutating func finishSegment(capturedMs: Int) throws {
        guard capturedMs >= 0, capturedMs <= 9_007_199_254_740_991 - offsetMs else { throw RecorderError.message("Captured audio duration is outside its supported range.") }
        offsetMs += capturedMs
    }
    public func chunk(startSeconds: Double, endSeconds: Double, text: String) throws -> TranscriptChunk {
        let maximum = Double(9_007_199_254_740_991 - offsetMs) / 1000
        guard startSeconds.isFinite, endSeconds.isFinite, startSeconds >= 0, endSeconds >= startSeconds, endSeconds <= maximum else { throw RecorderError.message("Speech returned an invalid audio timestamp.") }
        return TranscriptChunk(startMs: offsetMs + Int(startSeconds * 1000), endMs: offsetMs + Int(endSeconds * 1000), text: text)
    }
}

// Legacy authorization APIs invoke completion on an arbitrary queue. This bridge
// deliberately has no actor isolation; only the awaiting caller returns to its actor.
public enum PermissionCallback {
    public nonisolated static func result(_ request: @Sendable (@escaping @Sendable (Bool) -> Void) -> Void) async -> Bool {
        await withCheckedContinuation { continuation in
            request { @Sendable allowed in continuation.resume(returning: allowed) }
        }
    }
}

// Each resumed Apple analyzer starts a new textual segment. Its first result may
// omit a leading separator even when the preceding segment ended in a word or
// sentence. Add one before a new word; retain all supplied whitespace/punctuation.
public struct SpeechSegmentSpacing {
    let precedingFinalEnding: Character?
    private var isBeginning = true
    public init(precedingFinalEnding: Character?) { self.precedingFinalEnding = precedingFinalEnding }
    public mutating func text(_ original: String, isFinal: Bool) -> String {
        var text = original
        if isBeginning, let previous = precedingFinalEnding, !previous.isWhitespace,
           let first = original.first, first.isLetter || first.isNumber {
            text = " " + original
        }
        if isFinal && !original.isEmpty { isBeginning = false }
        return text
    }
}
