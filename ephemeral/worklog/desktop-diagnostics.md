# Desktop diagnostic logs

correction: macOS unified logging is a supported platform choice, but the app has no user-facing export and transient recording errors are cleared without a retained diagnostic record. Add bounded local logs and an ordinary export action; keep conversation text, microphone audio, credentials and provider state out of deliberate diagnostic capture.

decision: Native recording diagnostics and bundled server stderr share a rotating desktop log under ~/Library/Logs/Gimbal. A process lock serializes writes/rotation/export across concurrent development and installed instances. Export is a plain text snapshot of only the bounded diagnostic logs, not the project catalog or macOS-wide logs.

finding: The new log captured repeated transient "Transcript disconnected or waiting for saved text" followed by recovery during ordinary synthetic recording. RecorderBridge treats the current accepted sequence lag as display unavailability, so normal pending receipt can become a short-lived error. This reproduces the reported presentation symptom without proving it was the user's microphone error. Preserve this as a concrete follow-up rather than changing recorder semantics in the logging task.

decision: Export and Open Logs Folder live in the native Help menu and remain available when the bundled/local server cannot start. Logging and export do not depend on a functioning web workspace.
