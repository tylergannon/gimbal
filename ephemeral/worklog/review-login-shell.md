# Login shell review

decision: Review PR #503 at its merge commit, isolated from existing root-checkout edits. Review is not authorization to implement changes.
doc_bug: Browser login and server-restart evidence do not cover macOS shell authentication. Its main-frame origin restriction cancels the GitHub navigation, its WKWebView data store is nonpersistent, and native recording commands use a separate ephemeral URLSession without an authentication bridge.
decision: The authentication gate already attaches the customer-defined auth.User to Go request context. Application request-event Locals remain empty; expose that same verified object through the app-owned request boundary if direct event.Locals access is required, without serializing the customer object to the page.
decision: A macOS session credential should remain native-owned and scoped to its configured server. Review any Keychain persistence implementation together with native command authentication, callback handoff, logout, expiry, and server switching.
