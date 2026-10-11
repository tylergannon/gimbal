import Foundation
@_exported import TranscriptSource

struct TranscriptNormalizer {
  private(set) var boundary = 0
  private(set) var provisional: [TranscriptChunk] = []
  mutating func consume(_ event: TranscriptEvent) throws -> (
    finalized: [TranscriptChunk], provisional: [TranscriptChunk]
  ) {
    let chunk = event.chunk
    guard chunk.startMs >= boundary, chunk.endMs >= chunk.startMs else {
      throw RecorderError.message("Speech returned an invalid audio range.")
    }
    // A result replaces every overlapping volatile range, regardless of endpoints.
    provisional.removeAll {
      $0.endMs > chunk.startMs && $0.startMs < chunk.endMs || $0.startMs == chunk.startMs
    }
    if event.isFinal {
      boundary = chunk.endMs
      provisional.removeAll { $0.startMs < boundary }
      return ([chunk], provisional)
    }
    provisional.append(chunk)
    provisional.sort { $0.startMs < $1.startMs }
    return ([], provisional)
  }
}
@MainActor final class TranscriptSender {
  typealias Send = (Int, [TranscriptChunk], [TranscriptChunk]) async throws -> Void
  private let send: Send
  private var finals: [TranscriptChunk] = []
  private var tail: [TranscriptChunk] = []
  private var dirty = false
  private var running: Task<Void, Never>?
  private(set) var sequence = 0
  private(set) var failure: Error?
  var onFailure: ((Error) -> Void)?
  var onDelivery: ((Int, Bool) -> Void)?
  let pendingLimit = 256 * 1024
  let requestLimit = 48 * 1024
  init(send: @escaping Send) { self.send = send }
  func enqueue(finalized: [TranscriptChunk], provisional: [TranscriptChunk]) throws {
    guard failure == nil else { throw failure! }
    finals += finalized
    tail = provisional
    dirty = true
    onDelivery?(sequence, true)
    guard size(finals + tail) <= pendingLimit else {
      throw RecorderError.message(
        "Transcript delivery is behind and its memory limit was reached. The transcript may be incomplete."
      )
    }
    if running == nil { running = Task { await pump() } }
  }
  private func size(_ chunks: [TranscriptChunk]) -> Int {
    (try? JSONEncoder().encode(chunks).count) ?? Int.max
  }
  private func pump() async {
    while dirty && failure == nil && !Task.isCancelled {
      var batch: [TranscriptChunk] = []
      while let first = finals.first, size(batch + [first]) <= requestLimit / 2 {
        batch.append(first)
        finals.removeFirst()
      }
      if batch.isEmpty && !finals.isEmpty {
        fail(RecorderError.message("A transcript result exceeded the delivery size limit."))
        break
      }
      let frozenTail = finals.isEmpty ? tail : []
      guard size(batch + frozenTail) <= requestLimit else {
        fail(RecorderError.message("A provisional transcript exceeded the delivery size limit."))
        break
      }
      dirty = !finals.isEmpty
      let next = sequence + 1
      // Freeze once. Every retry uses the identical sequence and payload.
      let deadline = ContinuousClock.now.advanced(by: .seconds(8))
      var delivered = false
      while !delivered && ContinuousClock.now < deadline && !Task.isCancelled {
        do {
          try await send(next, batch, frozenTail)
          delivered = true
        } catch {
          if ContinuousClock.now >= deadline {
            fail(error)
            break
          }
          try? await Task.sleep(for: .milliseconds(250))
        }
      }
      if delivered {
        sequence = next
        onDelivery?(sequence, dirty)
      } else if failure == nil {
        fail(
          RecorderError.message("Transcript delivery timed out. The transcript may be incomplete."))
      }
    }
    running = nil
  }
  private func fail(_ error: Error) {
    failure = error
    onFailure?(error)
  }
  func flush() async throws {
    while let task = running { await task.value }
    if let failure { throw failure }
  }
  func cancel() {
    running?.cancel()
    running = nil
  }
}
