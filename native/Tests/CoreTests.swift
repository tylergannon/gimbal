import Foundation

@main struct CoreTests {
  @MainActor static func main() async throws {
    let allowed = await PermissionCallback.result { @Sendable completion in
      DispatchQueue.global(qos: .userInitiated).async {
        precondition(!Thread.isMainThread)
        completion(true)
      }
    }
    let denied = await PermissionCallback.result { @Sendable completion in
      DispatchQueue.global(qos: .utility).async {
        precondition(!Thread.isMainThread)
        completion(false)
      }
    }
    precondition(allowed && !denied)
    var resumedSpacing = SpeechSegmentSpacing(precedingFinalEnding: ".")
    precondition(resumedSpacing.text("The recording", isFinal: false) == " The recording")
    precondition(
      resumedSpacing.text("The recording has resumed.", isFinal: true)
        == " The recording has resumed.")
    precondition(resumedSpacing.text(" More words.", isFinal: true) == " More words.")
    var existingWhitespace = SpeechSegmentSpacing(precedingFinalEnding: " ")
    precondition(existingWhitespace.text("Resume", isFinal: true) == "Resume")
    var punctuation = SpeechSegmentSpacing(precedingFinalEnding: "d")
    precondition(punctuation.text(", then more", isFinal: false) == ", then more")
    var leadingWhitespace = SpeechSegmentSpacing(precedingFinalEnding: ".")
    precondition(leadingWhitespace.text("\n  Resume", isFinal: true) == "\n  Resume")
    var firstRecording = SpeechSegmentSpacing(precedingFinalEnding: nil)
    precondition(firstRecording.text("First words", isFinal: true) == "First words")
    var normalizer = TranscriptNormalizer()
    _ = try normalizer.consume(
      TranscriptEvent(chunk: .init(startMs: 0, endMs: 100, text: "hello"), isFinal: false))
    let revision = try normalizer.consume(
      TranscriptEvent(chunk: .init(startMs: 0, endMs: 150, text: "hello world "), isFinal: false))
    precondition(revision.provisional.count == 1 && revision.provisional[0].text == "hello world ")
    let final = try normalizer.consume(
      TranscriptEvent(chunk: .init(startMs: 0, endMs: 130, text: "Hello world. "), isFinal: true))
    precondition(final.finalized.count == 1 && final.provisional.isEmpty)
    do {
      _ = try normalizer.consume(
        TranscriptEvent(chunk: .init(startMs: 20, endMs: 90, text: "bad"), isFinal: true))
      fatalError("finalized rewrite accepted")
    } catch {}
    var calls: [(Int, [TranscriptChunk], [TranscriptChunk])] = []
    var lost = true
    let sender = TranscriptSender { sequence, finals, tail in
      calls.append((sequence, finals, tail))
      if lost {
        lost = false
        throw RecorderError.message("Lost response after apply")
      }
      try await Task.sleep(for: .milliseconds(20))
    }
    var delivery: [(Int, Bool)] = []
    sender.onDelivery = { delivery.append(($0, $1)) }
    try sender.enqueue(finalized: [], provisional: [.init(startMs: 0, endMs: 100, text: "old")])
    precondition(delivery.last?.0 == 0 && delivery.last?.1 == true)
    try await Task.sleep(for: .milliseconds(30))
    precondition(delivery.last?.1 == true)
    for i in 0..<1000 {
      try sender.enqueue(
        finalized: [], provisional: [.init(startMs: 0, endMs: 100, text: "revision \(i)")])
    }
    try sender.enqueue(finalized: [.init(startMs: 0, endMs: 100, text: "final ")], provisional: [])
    try await sender.flush()
    precondition(calls.count == 3)
    precondition(calls[0].0 == calls[1].0 && calls[0].2 == calls[1].2)
    precondition(calls[2].1[0].text == "final " && calls[2].2.isEmpty)
    precondition(sender.sequence == 2)
    precondition(delivery.last?.0 == 2 && delivery.last?.1 == false)
    let oversized = TranscriptSender { _, _, _ in }
    do {
      try oversized.enqueue(
        finalized: [.init(startMs: 0, endMs: 1, text: String(repeating: "x", count: 300_000))],
        provisional: [])
      fatalError("memory bound missing")
    } catch {}
    var batches: [[TranscriptChunk]] = []
    let split = TranscriptSender { _, finals, _ in batches.append(finals) }
    let manyFinals = (0..<100).map {
      TranscriptChunk(
        startMs: $0 * 100, endMs: ($0 + 1) * 100, text: String(repeating: "a", count: 1000))
    }
    try split.enqueue(finalized: manyFinals, provisional: [])
    try await split.flush()
    precondition(batches.count > 1 && batches.flatMap { $0 } == manyFinals)
    var errors = 0
    let failing = TranscriptSender { _, _, _ in throw RecorderError.message("Server unavailable") }
    failing.onFailure = { _ in errors += 1 }
    try failing.enqueue(finalized: [.init(startMs: 0, endMs: 1, text: "final")], provisional: [])
    let began = ContinuousClock.now
    do {
      try await failing.flush()
      fatalError("Unavailable server claimed success")
    } catch {}
    precondition(began.duration(to: .now) < .seconds(10) && errors == 1 && failing.sequence == 0)
    print(
      "PASS resumed speech spacing/revisions preserves whitespace/punctuation, background permission callbacks allowed/denied, bounded failure deadline, ordered final splitting, normalizer replacement/finality, frozen lost-response retry, 1000-partial coalescing, final preservation, pending memory bound"
    )
  }
}
