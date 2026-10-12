import SwiftUI

#if os(macOS)
  import AppKit
#endif

// Retained once by the generic application shell, including foreground recovery.
@MainActor final class ApplicationState: ObservableObject {
  #if os(macOS)
    let workspace = WorkspaceState()
  #endif
  let input: RecordingInput
  let controller: RecorderController?
  let commands: GeneratedCommands?
  private var heartbeat: Task<Void, Never>?
  #if os(macOS)
    private var viewers: [NSWindow] = []
  #endif

  init(host: NativeHost) {
    let input = RecordingInput()
    self.input = input
    let commands = host.url.map { GeneratedCommands(baseURL: $0) }
    self.commands = commands
    controller = commands.map { commands in
      RecorderController(
        commands: commands,
        makeSource: {
          if let error = input.configurationError { throw RecorderError.message(error) }
          if let fixture = input.fixtureURL { return SyntheticSource(fixtureURL: fixture) }
          return MicrophoneSource(selection: input.selection)
        }, sourceName: input.fixtureURL == nil ? "microphone" : "synthetic")
    }
    heartbeat = Task { [weak self] in
      while !Task.isCancelled {
        try? await Task.sleep(for: .seconds(5))
        guard let self, let controller = self.controller else { return }
        try? await self.commands?.heartbeat(
          sessionID: controller.status.sessionID, state: controller.status.state)
      }
    }
    host.onShutdown = { [weak self] in
      self?.heartbeat?.cancel()
      self?.controller?.shutdown()
    }
    host.onBackgroundChange = { [weak self] backgrounded in
      self?.controller?.setAppBackgrounded(backgrounded)
    }
    host.onAvailabilityChange = { [weak self] available, message in
      // Only the WebKit receipt can confirm that transcript text is visible.
      if !available { self?.controller?.displayAvailability(false, error: message) }
    }
  }
  #if os(macOS)
    func openViewer(path: String, origin: URL) {
      guard path.hasPrefix("/projects/"),
        let url = URL(string: path, relativeTo: origin)?.absoluteURL
      else { return }
      let window = NSWindow(
        contentRect: NSRect(x: 0, y: 0, width: 800, height: 650),
        styleMask: [.titled, .closable, .resizable, .miniaturizable], backing: .buffered,
        defer: false)
      window.title = "Gimbal document"
      window.isReleasedWhenClosed = false
      window.contentViewController = NSHostingController(
        rootView: ApplicationWebView(url: url, recoveryRevision: 0))
      viewers.append(window)
      window.center()
      window.makeKeyAndOrderFront(nil)
    }
  #endif
}

struct ApplicationView: View {
  @ObservedObject var state: ApplicationState
  let url: URL
  let recoveryRevision: Int
  var body: some View {
    if let controller = state.controller {
      RecorderScreen(
        controller: controller, input: state.input, state: state, url: url,
        recoveryRevision: recoveryRevision)
    } else {
      Text("The recorder could not connect to the local application server.").padding()
    }
  }
}

#if os(macOS)
  struct ApplicationSettingsView: View {
    @ObservedObject var state: ApplicationState
    var body: some View {
      RecordingSettings(input: state.input, controller: state.controller)
    }
  }
#endif
