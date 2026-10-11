import XCTest
import TranscriptSource
@testable import AppleTranscription
import Foundation
import AVFoundation
import Speech
import CoreAudio

// Initialized once, read only during the awaited background conversion check.
final class ReadOnlyAudioBuffer: @unchecked Sendable {
    let buffer: AVAudioPCMBuffer
    init(_ buffer: AVAudioPCMBuffer) { self.buffer = buffer }
}
final class AudioTests: XCTestCase {
    func testLevelUsesAllChannelsAndPCMFormats() {
        let stereo = AVAudioFormat(standardFormatWithSampleRate: 8000, channels: 2)!
        let buffer = AVAudioPCMBuffer(pcmFormat: stereo, frameCapacity: 4)!
        buffer.frameLength = 4
        for channel in 0..<2 { for frame in 0..<4 { buffer.floatChannelData![channel][frame] = 0 } }
        XCTAssertEqual(AudioInputLevel.normalized(buffer), 0)
        for frame in 0..<4 { buffer.floatChannelData![1][frame] = 1 }
        // Half the samples are full scale: RMS sqrt(0.5), about -3 dBFS.
        XCTAssertEqual(AudioInputLevel.normalized(buffer), Float((10 * log10(0.5) + 60) / 60), accuracy: 0.0001)
        for channel in 0..<2 { for frame in 0..<4 { buffer.floatChannelData![channel][frame] = 0.001 } }
        XCTAssertEqual(AudioInputLevel.normalized(buffer), 0, accuracy: 0.0001)
        let interleaved = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: 8000, channels: 2, interleaved: true)!
        let integers = AVAudioPCMBuffer(pcmFormat: interleaved, frameCapacity: 4)!
        integers.frameLength = 4
        for index in 0..<8 { integers.int16ChannelData![0][index] = index.isMultiple(of: 2) ? 0 : Int16.min }
        XCTAssertEqual(AudioInputLevel.normalized(integers), AudioInputLevel.normalized(bufferWithHalfPower()), accuracy: 0.0001)
        func bufferWithHalfPower() -> AVAudioPCMBuffer {
            for frame in 0..<4 { buffer.floatChannelData![0][frame] = 0; buffer.floatChannelData![1][frame] = 1 }
            return buffer
        }
        buffer.frameLength = 0
        XCTAssertEqual(AudioInputLevel.normalized(buffer), 0)
    }
    @MainActor func testIntakeLevelsAreThrottledAndClosedAdmissionIsSilent() throws {
        let format = AVAudioFormat(standardFormatWithSampleRate: 8000, channels: 1)!
        let buffer = AVAudioPCMBuffer(pcmFormat: format, frameCapacity: 400)!
        buffer.frameLength = 400
        for frame in 0..<400 { buffer.floatChannelData![0][frame] = 0.5 }
        let (stream, continuation) = AsyncStream<AnalyzerInput>.makeStream()
        let output = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: 16000, channels: 1, interleaved: false)!
        final class Levels: @unchecked Sendable {
            var values: [Float] = [] // Accessed synchronously in this test.
        }
        let levels = Levels()
        let intake = try AudioIntake(input: format, output: output, continuation: continuation, onLevel: { levels.values.append($0) })
        intake.receive(buffer); intake.receive(buffer); intake.receive(buffer)
        XCTAssertEqual(levels.values.count, 2) // 50ms then 150ms; no 100ms update.
        XCTAssertTrue(levels.values.allSatisfy { abs($0 - 0.899657) < 0.0001 })
        _ = try intake.close()
        intake.receive(buffer)
        XCTAssertEqual(levels.values.count, 2)
        withExtendedLifetime(stream) {}
    }
    @MainActor func testMissingPinnedMicrophoneDoesNotFallBack() throws {
        func defaultInput() throws -> AudioDeviceID {
            var address = AudioObjectPropertyAddress(mSelector: kAudioHardwarePropertyDefaultInputDevice, mScope: kAudioObjectPropertyScopeGlobal, mElement: kAudioObjectPropertyElementMain)
            var id = AudioDeviceID(0)
            var size = UInt32(MemoryLayout<AudioDeviceID>.size)
            XCTAssertEqual(AudioObjectGetPropertyData(AudioObjectID(kAudioObjectSystemObject), &address, 0, nil, &size, &id), noErr)
            return id
        }
        let originalDefault = try defaultInput()
        let selection = InputDeviceSelection.device(uid: "voice-notes-nonexistent-input-uid")
        let source = MicrophoneSource(selection: selection)
        XCTAssertEqual(source.selection, selection)
        XCTAssertThrowsError(try InputDeviceInventory.apply(selection, engine: AVAudioEngine())) { error in
            XCTAssertTrue(error.localizedDescription.contains("selected microphone is unavailable"))
        }
        XCTAssertEqual(try defaultInput(), originalDefault)
    }
    @MainActor func testSourceContract() async throws {
        let input = AVAudioFormat(standardFormatWithSampleRate: 8000, channels: 2)!
        let output = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: 16000, channels: 1, interleaved: false)!
        let (stream, continuation) = AsyncStream<AnalyzerInput>.makeStream()
        let intake = try AudioIntake(input: input, output: output, continuation: continuation)
        let buffer = AVAudioPCMBuffer(pcmFormat: input, frameCapacity: 800)!
        buffer.frameLength = 800
        for channel in 0..<2 { for frame in 0..<800 { buffer.floatChannelData![channel][frame] = Float(sin(Double(frame) * 0.1)) } }
        let readOnly = ReadOnlyAudioBuffer(buffer)
        await withCheckedContinuation { continuation in
            DispatchQueue.global(qos: .userInitiated).async {
                precondition(!Thread.isMainThread)
                intake.receive(readOnly.buffer)
                intake.receive(readOnly.buffer)
                continuation.resume()
            }
        }
        let duration = try intake.close()
        intake.receive(buffer) // Closed intake cannot admit another buffer.
        var count = 0
        var previous = -1.0
        for await item in stream {
            let time = item.bufferStartTime!.seconds
            precondition(time > previous); previous = time
            precondition(item.bufferFormat.sampleRate == 16000 && item.bufferFormat.channelCount == 1)
            count += 1
        }
        precondition(count >= 2 && duration == 200)
        let (limitedStream, limitedContinuation) = AsyncStream<AnalyzerInput>.makeStream(bufferingPolicy: .bufferingNewest(1))
        let limited = try AudioIntake(input: input, output: output, continuation: limitedContinuation)
        withExtendedLifetime(limitedStream) { limited.receive(buffer); limited.receive(buffer) }
        do { _ = try limited.close(); fatalError("Audio queue drop must fail visibly") } catch {}
        let changedInput = AVAudioFormat(standardFormatWithSampleRate: 48000, channels: 1)!
        let changedBuffer = AVAudioPCMBuffer(pcmFormat: changedInput, frameCapacity: 4800)!
        changedBuffer.frameLength = 4800
        for frame in 0..<4800 { changedBuffer.floatChannelData![0][frame] = Float(sin(Double(frame) * 0.1)) }
        let (changedStream, changedContinuation) = AsyncStream<AnalyzerInput>.makeStream()
        let changing = try AudioIntake(input: input, output: output, continuation: changedContinuation)
        changing.receive(buffer); changing.receive(changedBuffer)
        let changedDuration = try changing.close()
        precondition(changedDuration == 200)
        var convertedFrames: AVAudioFrameCount = 0
        for await item in changedStream { convertedFrames += AVAudioFrameCount((item.bufferDuration.seconds * item.bufferFormat.sampleRate).rounded()) }
        precondition(convertedFrames == 3200) // No resampler padding enters audio time.
        let watchedEngine = AVAudioEngine()
        let watch = AudioConfigurationWatch()
        var changes = 0
        watch.start(engine: watchedEngine) { changes += 1 }
        await withCheckedContinuation { continuation in
            DispatchQueue.global().async {
                NotificationCenter.default.post(name: .AVAudioEngineConfigurationChange, object: watchedEngine)
                continuation.resume()
            }
        }
        for _ in 0..<100 where changes == 0 { try await Task.sleep(for: .milliseconds(1)) }
        precondition(changes == 1)
        NotificationCenter.default.post(name: .AVAudioEngineConfigurationChange, object: watchedEngine)
        try await Task.sleep(for: .milliseconds(5)); precondition(changes == 1)
        watch.start(engine: watchedEngine) { changes += 1 }
        NotificationCenter.default.post(name: .AVAudioEngineConfigurationChange, object: watchedEngine)
        watch.invalidate() // Queued callback from the old segment is stale.
        watch.start(engine: watchedEngine) { changes += 10 }
        try await Task.sleep(for: .milliseconds(5)); precondition(changes == 1)
        watch.invalidate()
        NotificationCenter.default.post(name: .AVAudioEngineConfigurationChange, object: watchedEngine)
        try await Task.sleep(for: .milliseconds(5)); precondition(changes == 1)
        var timeline = CapturedAudioTimeline()
        try timeline.finishSegment(capturedMs: duration)
        let resumed = try timeline.chunk(startSeconds: 0.05, endSeconds: 0.15, text: "resumed")
        precondition(resumed.startMs == 250 && resumed.endMs == 350)
        // Wall-clock pause time has no input into this captured-frame timeline.
        try timeline.finishSegment(capturedMs: 300)
        let third = try timeline.chunk(startSeconds: 0, endSeconds: 0.1, text: "third")
        precondition(third.startMs == 500 && third.endMs == 600)
        do { _ = try timeline.chunk(startSeconds: 0, endSeconds: .infinity, text: "invalid"); fatalError("Invalid timestamp accepted") } catch {}
        print("PASS background audio configuration notification/stale callback/expected teardown, input format replacement preserves200ms without padding, resume offsets exclude pause, bounded audio queue overflow fails, stereo8k→mono16k conversion, ordered audio-relative timestamps, closed intake rejects buffers; duration \(duration)ms")
    }
}
