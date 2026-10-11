import XCTest
import TranscriptSource
@testable import ScriptedTranscription
import Foundation
final class SyntheticTests: XCTestCase {
    @MainActor func testSimulatedLevelsFollowPlaybackAndClearOnPause() async throws {
        var now = ContinuousClock.now
        let source = SyntheticSource(events: [.init(atMs: 10000, startMs: 0, endMs: 10000, text: "later", isFinal: true)])
        source.now = { now }
        var levels: [Float] = []
        source.onLevel = { levels.append($0) }
        try await source.start()
        XCTAssertEqual(levels, [0])
        now = now.advanced(by: .milliseconds(550)); source.playbackTick()
        XCTAssertEqual(levels.last, 0.9)
        source.playbackTick(); XCTAssertEqual(levels.count, 2)
        source.stopAdmission(); XCTAssertEqual(levels.last, 0)
        let pausedCount = levels.count
        now = now.advanced(by: .seconds(20)); source.playbackTick()
        try await Task.sleep(for: .milliseconds(50))
        XCTAssertEqual(levels.count, pausedCount)
        try await source.start(); XCTAssertEqual(levels.last, 0.9)
        now = now.advanced(by: .milliseconds(350)); source.playbackTick()
        XCTAssertEqual(levels.last, 0)
        source.cancel(); XCTAssertEqual(levels.last, 0)
        XCTAssertNil(source.onLevel)
    }
    @MainActor func testOrdinaryFinalDoesNotHideInvalidLaterRevision() async throws {
        let source = SyntheticSource(events: [
            .init(atMs: 0, startMs: 0, endMs: 100, text: "final", isFinal: true),
            .init(atMs: 0, startMs: 0, endMs: 150, text: "invalid revision", isFinal: false)
        ])
        var received: [TranscriptEvent] = []
        source.onEvent = { received.append($0) }
        try await source.prepare(); try await source.start(); source.stopAdmission()
        XCTAssertEqual(received.map(\.chunk.text), ["final", "invalid revision"])
        source.cancel()
    }
    @MainActor func testDrainFinalizesEveryRevisedRangeInOrder() async throws {
        let source = SyntheticSource(events: [
            .init(atMs: 0, startMs: 0, endMs: 100, text: "first", isFinal: false),
            .init(atMs: 0, startMs: 200, endMs: 300, text: "second", isFinal: false),
            .init(atMs: 0, startMs: 0, endMs: 150, text: "revised", isFinal: false)
        ])
        var received: [TranscriptEvent] = []
        source.onEvent = { received.append($0) }
        try await source.prepare(); try await source.start(); try await source.drain()
        XCTAssertEqual(received.filter(\.isFinal).map(\.chunk.text), ["revised", "second"])
        source.cancel()
    }
    @MainActor func testExplicitFixtureURL() async throws {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString + ".json")
        defer { try? FileManager.default.removeItem(at: url) }
        try JSONEncoder().encode([SyntheticSource.Fixture(atMs: 0, startMs: 0, endMs: 100, text: "fixture", isFinal: true)]).write(to: url)
        let source = SyntheticSource(fixtureURL: url)
        var received: [TranscriptEvent] = []
        source.onEvent = { received.append($0) }
        try await source.prepare(); try await source.start(); try await source.drain()
        XCTAssertEqual(received.map(\.chunk.text), ["fixture"])
        source.cancel()
    }
    @MainActor func testSourceContract() async throws {
        var now = ContinuousClock.now
        let source = SyntheticSource(events: [
            .init(atMs: 1000, startMs: 0, endMs: 1000, text: "partial ", isFinal: false),
            .init(atMs: 2000, startMs: 0, endMs: 1900, text: "revised ", isFinal: true),
            .init(atMs: 4000, startMs: 3000, endMs: 3900, text: "next ", isFinal: true)
        ])
        source.now = { now }
        var events: [TranscriptEvent] = []
        source.onEvent = { events.append($0) }
        try await source.prepare(); try await source.start()
        now = now.advanced(by: .milliseconds(1200))
        source.stopAdmission()
        now = now.advanced(by: .seconds(100)) // Stop request freezes time before asynchronous drain.
        precondition(events.count == 1)
        try await source.drain()
        precondition(events.count == 2 && events[0].isFinal == false && events[1].isFinal)
        now = now.advanced(by: .seconds(100)) // Pause time must not advance fixture.
        precondition(events.count == 2)
        try await source.start()
        now = now.advanced(by: .milliseconds(1000)); try await source.drain()
        precondition(events.count == 2) // Revision of pause-finalized range skipped.
        try await source.start()
        now = now.advanced(by: .milliseconds(2000)); try await source.drain()
        precondition(events.count == 3 && events[2].chunk.text == "next ")
        source.cancel()
        print("PASS synchronous admission seal freezes synthetic clock before drain and across100s pause, partial finalized on drain, resume retains timeline, late revision cannot rewrite final")
    }
}
