import Foundation
import AVFoundation
import TranscriptSource
#if os(macOS)
import CoreAudio
import AudioToolbox
#endif

public enum InputDeviceSelection: Equatable, Sendable {
    case systemDefault
    case device(uid: String)
}
public struct InputDevice: Equatable, Sendable {
    public let uid: String
    public let name: String
    public init(uid: String, name: String) { self.uid = uid; self.name = name }
}
@MainActor public enum InputDeviceInventory {
    #if os(macOS)
    private static func address(_ selector: AudioObjectPropertySelector, scope: AudioObjectPropertyScope = kAudioObjectPropertyScopeGlobal) -> AudioObjectPropertyAddress {
        AudioObjectPropertyAddress(mSelector: selector, mScope: scope, mElement: kAudioObjectPropertyElementMain)
    }
    private static func deviceIDs() throws -> [AudioDeviceID] {
        var property = address(kAudioHardwarePropertyDevices)
        var size: UInt32 = 0
        guard AudioObjectGetPropertyDataSize(AudioObjectID(kAudioObjectSystemObject), &property, 0, nil, &size) == noErr else { throw RecorderError.message("Microphone inventory is unavailable.") }
        guard size > 0 else { return [] }
        var ids = [AudioDeviceID](repeating: 0, count: Int(size) / MemoryLayout<AudioDeviceID>.size)
        let status = ids.withUnsafeMutableBytes { AudioObjectGetPropertyData(AudioObjectID(kAudioObjectSystemObject), &property, 0, nil, &size, $0.baseAddress!) }
        guard status == noErr else { throw RecorderError.message("Microphone inventory could not be read.") }
        return ids
    }
    private static func string(_ id: AudioDeviceID, _ selector: AudioObjectPropertySelector) -> String? {
        var property = address(selector)
        var value: Unmanaged<CFString>?
        var size = UInt32(MemoryLayout<Unmanaged<CFString>?>.size)
        guard AudioObjectGetPropertyData(id, &property, 0, nil, &size, &value) == noErr else { return nil }
        return value?.takeUnretainedValue() as String?
    }
    private static func hasInput(_ id: AudioDeviceID) -> Bool {
        var property = address(kAudioDevicePropertyStreamConfiguration, scope: kAudioDevicePropertyScopeInput)
        var size: UInt32 = 0
        guard AudioObjectGetPropertyDataSize(id, &property, 0, nil, &size) == noErr, size > 0 else { return false }
        let bytes = UnsafeMutableRawPointer.allocate(byteCount: Int(size), alignment: MemoryLayout<AudioBufferList>.alignment)
        defer { bytes.deallocate() }
        guard AudioObjectGetPropertyData(id, &property, 0, nil, &size, bytes) == noErr else { return false }
        return UnsafeMutableAudioBufferListPointer(bytes.assumingMemoryBound(to: AudioBufferList.self)).contains { $0.mNumberChannels > 0 }
    }
    public static func inputs() throws -> [InputDevice] {
        try deviceIDs().filter(hasInput).compactMap { id in
            guard let uid = string(id, kAudioDevicePropertyDeviceUID), let name = string(id, kAudioObjectPropertyName) else { return nil }
            return InputDevice(uid: uid, name: name)
        }.sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
    }
    static func apply(_ selection: InputDeviceSelection, engine: AVAudioEngine) throws {
        guard case let .device(uid) = selection else { return }
        guard let id = try deviceIDs().first(where: { hasInput($0) && string($0, kAudioDevicePropertyDeviceUID) == uid }) else { throw RecorderError.message("The selected microphone is unavailable. Choose another microphone before recording.") }
        guard let unit = engine.inputNode.audioUnit else { throw RecorderError.message("The microphone audio unit is unavailable.") }
        var selected = id
        guard AudioUnitSetProperty(unit, kAudioOutputUnitProperty_CurrentDevice, kAudioUnitScope_Global, 0, &selected, UInt32(MemoryLayout<AudioDeviceID>.size)) == noErr else { throw RecorderError.message("The selected microphone could not be opened.") }
        var actual = AudioDeviceID(0)
        var size = UInt32(MemoryLayout<AudioDeviceID>.size)
        guard AudioUnitGetProperty(unit, kAudioOutputUnitProperty_CurrentDevice, kAudioUnitScope_Global, 0, &actual, &size) == noErr, actual == selected else { throw RecorderError.message("The audio engine did not select the requested microphone.") }

    }
    #else
    static func configureRecordingSession(_ session: AVAudioSession) throws {
        // HFP input is opt-in. Use the same route policy for browsing and capture,
        // so starting a recording does not remove Bluetooth choices from inventory.
        try session.setCategory(.record, mode: .default, options: [.allowBluetoothHFP])
    }
    public static func inputs() throws -> [InputDevice] {
        let session = AVAudioSession.sharedInstance()
        // Configure the recording category without activating capture or interrupting playback.
        try configureRecordingSession(session)
        return (session.availableInputs ?? []).map { InputDevice(uid: $0.uid, name: $0.portName) }
    }
    static func apply(_ selection: InputDeviceSelection, session: AVAudioSession) throws {
        switch selection {
        case .systemDefault: try session.setPreferredInput(nil)
        case let .device(uid):
            guard let input = session.availableInputs?.first(where: { $0.uid == uid }) else { throw RecorderError.message("The selected microphone is unavailable.") }
            try session.setPreferredInput(input)
        }
    }
    #endif
}
