# Transcription readiness

This standalone Swift package contains the native transcript source contract,
Apple microphone capture/transcription, and explicit scripted playback. It has
no Go, web UI, or app bundle dependency. Sources deliver timecoded partial/final
events and share prepare/start/stopAdmission/drain/cancel lifecycle.

`TranscriptSource` exports the event/chunk types, lifecycle protocol, error and
captured-audio helpers. `AppleTranscription` exports `MicrophoneSource`,
`InputDeviceInventory.inputs()` and `InputDeviceSelection`. `ScriptedTranscription`
exports `SyntheticSource(events:)` and `SyntheticSource(fixtureURL:)`. Each source
instance represents one recording session and can be paused/drained/resumed.
The app retains normalization and transcript transport.

Sources optionally deliver `onLevel` on the main actor, normalized to 0...1.
The microphone source measures RMS power from admitted PCM in its existing tap,
maps -60...0 dBFS to the meter range, and emits at most one update per 100 ms of
captured audio. Pause/Stop/cancel clear the level synchronously and invalidate
queued callbacks. Samples are not retained. Scripted playback emits explicitly
simulated amplitude, including silence, using its pause-aware fixture clock.

`MicrophoneSource(selection: .systemDefault)` uses the current default input.
`.device(uid: uid)` pins a stable input UID across resumes. The source never
changes the system default and rejects a missing selected input. A live engine
configuration/route change ends recording with a failure callback. On iOS the
selection uses available inputs and the session's preferred input. Inventory on
iOS is meaningful after audio-session setup; its initial list may be empty.

Run independently:

```sh
./scripts/check-transcription.sh
```

These package checks use synthetic audio and scripted events; they do not launch
microphone capture or request permission. Run from the generated application's
root (or this canonical `recorder/` directory). The repository's broader focused
check is `./scripts/check-recorder.sh`, run from the repository root.

Separately, explicitly invoke the bounded real probe when recognition output is
intended:

```sh
./scripts/probe-transcription.sh --list-inputs
./scripts/probe-transcription.sh --input default --duration 15
./scripts/probe-transcription.sh --input DEVICE_UID --duration 15
```

On macOS, capture launches the probe's own app bundle through LaunchServices so
permission belongs to its identity. The canonical bundle is
`dev.skgo.recorder.transcription-probe` / `Recorder Transcription Probe`; template
application replaces both strings with the consumer's derived probe identity.
The script obtains the app name from `ProbeInfo.plist`.

The probe requests microphone/speech permission and explicitly prints recognized
text with captured-audio millisecond ranges. Use it only when transcript output
is intended. Capture is limited to 1–120 seconds; preparation plus capture and
drain has an absolute duration+30 second deadline. The macOS probe binary embeds microphone and speech usage descriptions in its
`__TEXT,__info_plist` section. Permission or asset preparation still depends on
the local authorization and device environment; the probe does not establish the
app's permission behavior. Tests convert synthetic
audio buffers and exercise source lifecycle; they do not prove microphone hardware
or Apple recognition accuracy. macOS 27+ and iOS 26+ are required.
