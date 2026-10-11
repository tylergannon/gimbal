import AVFoundation
import Foundation

// Inspect admitted PCM while the original tap buffer is alive. No samples are
// retained. Average power across all channels, with -60 dBFS as the meter floor.
enum AudioInputLevel {
    static func normalized(_ buffer: AVAudioPCMBuffer) -> Float {
        guard buffer.frameLength > 0 else { return 0 }
        var squareSum = 0.0
        var count = 0
        for audio in UnsafeMutableAudioBufferListPointer(buffer.mutableAudioBufferList) {
            guard let data = audio.mData else { continue }
            let samples = Int(buffer.frameLength) * Int(audio.mNumberChannels)
            func accumulate<T>(_ type: T.Type, bytes: Int, sample: (T) -> Double) {
                let available = min(samples, Int(audio.mDataByteSize) / bytes)
                let values = data.assumingMemoryBound(to: T.self)
                for index in 0..<available {
                    let value = sample(values[index])
                    if value.isFinite { squareSum += value * value }
                }
                count += available
            }
            switch buffer.format.commonFormat {
            case .pcmFormatFloat32: accumulate(Float.self, bytes: 4) { Double($0) }
            case .pcmFormatFloat64: accumulate(Double.self, bytes: 8) { $0 }
            case .pcmFormatInt16: accumulate(Int16.self, bytes: 2) { Double($0) / 32768 }
            case .pcmFormatInt32: accumulate(Int32.self, bytes: 4) { Double($0) / 2_147_483_648 }
            default: return 0
            }
        }
        guard count > 0, squareSum > 0 else { return 0 }
        let decibels = 10 * log10(squareSum / Double(count))
        return Float(max(0, min(1, (decibels + 60) / 60)))
    }
}
