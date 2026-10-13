# Worklog: native requirements recording (plan §4 Swift, §9 step 8)

## Done
- `RecordingCommands.open(sessionID:source:requirements:)`;
  `RecorderController.record(requirements: Bool = false)` passes it to open.
- `GeneratedCommands.open` sends `OpenRequest(..., requirements:)`.
- Recording menu: `Record` now `{ controller.record() }`; new
  `Record Requirements Discussion` directly below it, disabled with the same
  `canRecord` gate. No shortcut (Record owns ⇧⌘R). Status-popover/record
  control unchanged apart from the closure fix (plan defers it).
- Doubles record each open's flag; ControllerTests asserts `[false, true]`
  across a plain then requirements recording; ScriptedControllerTests asserts
  `[false]` for plain Record and `[true]` for `record(requirements: true)`.

## Proof
- `scripts/check-recorder.sh`: exit 0, all PASS lines.
- `go tool skgo native build --platform macos --preset synthetic`: BUILD SUCCEEDED.
- Live: scratch project served by `bin/gimbal --port 18741`; a scratch Swift
  package (not committed) compiled the app's own TranscriptCore,
  RecorderController, NativeCommands, Generated.swift, RecordingInput and the
  synthetic source, and drove `record(requirements: true)` then `record()`
  through `GeneratedCommands` with the bundled synthetic fixture. Resulting
  `.gimbal/recordings/*.meta.json`: `"requirements":true` (sequence 5) and
  `"requirements":false` (sequence 3).

## Friction
- Computer-use tools failed every call ("PreToolUse hook did not respond"),
  so the synthetic app was launched (`GIMBAL_SERVER_URL=...`) but its menu was
  not clicked. `osascript` System Events menu listing hung (likely a pending
  Automation/Accessibility prompt) and was killed. The menu click itself is
  unverified by a human-visible run; the path beneath it is.
- swift-format (`xcrun swift-format`) reflowed the two long lines; the touched
  files are otherwise swift-format clean.
- `go tool skgo native build` rewrites `internal/skgo/**/skgo_gen.go` (import
  order) and deletes `web/src/lib/{observation,workflow}/types.ts`; restored
  with `git checkout`. Check `git status` after a native build.
