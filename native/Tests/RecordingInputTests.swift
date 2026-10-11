import Foundation
import AppleTranscription

@main struct RecordingInputTests {
    static func fixtureBundle(info: [String: Any], resources: [String]) throws -> Bundle {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString + ".bundle")
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        var values = info
        values["CFBundleIdentifier"] = "dev.skgo.tests." + UUID().uuidString
        let plist = try PropertyListSerialization.data(fromPropertyList: values, format: .xml, options: 0)
        try plist.write(to: root.appendingPathComponent("Info.plist"))
        for resource in resources { try Data("[]".utf8).write(to: root.appendingPathComponent(resource)) }
        guard let bundle = Bundle(url: root) else { throw NSError(domain: "FixtureBundle", code: 1) }
        return bundle
    }

    @MainActor static func main() throws {
        var inventoryCalls = 0
        let inventory: @MainActor () throws -> [InputDevice] = {
            inventoryCalls += 1
            return [InputDevice(uid: "test-input", name: "Test microphone")]
        }
        for value in ["fake", "synthetci", "", "REAL"] {
            let invalid = RecordingInput(environment: ["VN_SOURCE": value], listInputs: inventory)
            precondition(invalid.configurationError != nil && invalid.error == invalid.configurationError)
            precondition(invalid.fixtureURL == nil && invalid.devices.isEmpty)
            invalid.refresh()
            precondition(inventoryCalls == 0 && invalid.error == invalid.configurationError)
        }
        let scripted = RecordingInput(environment: ["VN_SOURCE": "synthetic"], listInputs: inventory)
        precondition(scripted.fixtureURL != nil && scripted.configurationError == nil)
        scripted.refresh()
        precondition(inventoryCalls == 0)
        // An explicit fixture wins even when VN_SOURCE is invalid.
        let fixture = RecordingInput(environment: ["VN_SOURCE": "fake", "VN_TRANSCRIPT_FIXTURE": "/tmp/explicit-fixture.json"], listInputs: inventory)
        precondition(fixture.fixtureURL?.path == "/tmp/explicit-fixture.json" && fixture.configurationError == nil)
        precondition(inventoryCalls == 0)
        let defaultBundle = try fixtureBundle(info: ["VNDefaultSource": "synthetic"], resources: ["synthetic-transcript.json"])
        defer { try? FileManager.default.removeItem(at: defaultBundle.bundleURL) }
        let defaultFixture = RecordingInput(environment: [:], bundle: defaultBundle, listInputs: inventory)
        precondition(defaultFixture.fixtureURL == defaultBundle.url(forResource: "synthetic-transcript", withExtension: "json") && defaultFixture.configurationError == nil)
        let burstBundle = try fixtureBundle(info: ["VNDefaultSource": "synthetic", "VNTranscriptFixture": "burst-transcript"], resources: ["burst-transcript.json"])
        defer { try? FileManager.default.removeItem(at: burstBundle.bundleURL) }
        let burstFixture = RecordingInput(environment: [:], bundle: burstBundle, listInputs: inventory)
        precondition(burstFixture.fixtureURL == burstBundle.url(forResource: "burst-transcript", withExtension: "json") && burstFixture.configurationError == nil)
        let explicit = RecordingInput(environment: ["VN_TRANSCRIPT_FIXTURE": "/tmp/explicit-fixture.json"], bundle: burstBundle, listInputs: inventory)
        precondition(explicit.fixtureURL?.path == "/tmp/explicit-fixture.json")
        let missingBundle = try fixtureBundle(info: ["VNDefaultSource": "synthetic", "VNTranscriptFixture": "absent-fixture"], resources: [])
        defer { try? FileManager.default.removeItem(at: missingBundle.bundleURL) }
        let missingFixture = RecordingInput(environment: [:], bundle: missingBundle, listInputs: inventory)
        precondition(missingFixture.fixtureURL?.lastPathComponent == "missing-absent-fixture.json" && missingFixture.configurationError == nil)
        precondition(!FileManager.default.fileExists(atPath: missingFixture.fixtureURL!.path))
        for name in ["", "../fixture", "nested/fixture", "\\fixture", ".", "..", "fixture.json"] {
            let invalidBundle = try fixtureBundle(info: ["VNDefaultSource": "synthetic", "VNTranscriptFixture": name], resources: [])
            defer { try? FileManager.default.removeItem(at: invalidBundle.bundleURL) }
            let invalidFixture = RecordingInput(environment: [:], bundle: invalidBundle, listInputs: inventory)
            precondition(invalidFixture.fixtureURL == nil && invalidFixture.configurationError != nil && invalidFixture.error == invalidFixture.configurationError)
            invalidFixture.refresh()
        }
        precondition(inventoryCalls == 0)
        let real = RecordingInput(environment: ["VN_SOURCE": "real", "VN_INPUT_DEVICE": "test-input"], listInputs: inventory)
        precondition(real.fixtureURL == nil && real.configurationError == nil && real.error == nil)
        precondition(real.selection == .device(uid: "test-input") && real.devices.count == 1 && inventoryCalls == 1)
        real.refresh()
        precondition(inventoryCalls == 2)
        print("PASS default/named/missing/invalid bundle fixtures preserve synthetic routing and environment precedence; invalid sources never inventory microphones, synthetic and explicit fixtures avoid inventory, real selection retains pinned UID")
    }
}
