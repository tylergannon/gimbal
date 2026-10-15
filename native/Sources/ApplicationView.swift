import SwiftUI

#if os(macOS)
  import AppKit
#endif

// Retained once by the generic application shell, including foreground recovery.
@MainActor final class ApplicationState: ObservableObject {
  #if os(macOS)
    let workspace = WorkspaceState()
    private(set) var projectSwitching: ProjectSwitching?
    private var recordingStatus: RecordingStatusItem?
  #endif
  let input: RecordingInput
  let controller: RecorderController?
  let commands: GeneratedCommands?
  let origin: URL?
  #if os(macOS)
    @Published var settingsPane = "recording"
    @Published var settingsProject = ""
    var requestSettings: ((String, String) -> Void)?
  #endif
  private var heartbeat: Task<Void, Never>?
  #if os(macOS)
    private var viewers: [NSWindow] = []
  #endif

  init(host: NativeHost) {
    let input = RecordingInput()
    self.input = input
    origin = host.url
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
    #if os(macOS)
      if let controller, let commands {
        let switching = ProjectSwitching(controller: controller, commands: commands, workspace: workspace)
        projectSwitching = switching
        workspace.requestProject = { [weak switching] in switching?.request(path: $0) }
        recordingStatus = RecordingStatusItem(
          controller: controller, input: input, workspace: workspace,
          recordingProject: { [weak self] in self?.commands?.recordingProject ?? "" },
          openTranscript: { [weak self] in
            guard let workspace = self?.workspace, let window = workspace.webView?.window else {
              return
            }
            window.deminiaturize(nil)
            window.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            workspace.send("openRecording")
          },
          switchProject: { [weak self] in
            guard let workspace = self?.workspace, let window = workspace.webView?.window else {
              return
            }
            window.deminiaturize(nil)
            window.makeKeyAndOrderFront(nil)
            NSApp.activate(ignoringOtherApps: true)
            self?.projectSwitching?.showMenu()
          })
      }
    #endif
    heartbeat = Task { [weak self] in
      while !Task.isCancelled {
        try? await Task.sleep(for: .seconds(5))
        guard let self, let controller = self.controller else { return }
        try? await self.commands?.heartbeat(
          sessionID: controller.status.sessionID, state: controller.status.state,
          durationMs: Int(controller.recordingDuration.elapsed() * 1000))
      }
    }
    host.onShutdown = { [weak self] in
      self?.heartbeat?.cancel()
      #if os(macOS)
        self?.recordingStatus?.shutdown()
      #endif
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
    func showRecordingControls() { recordingStatus?.showControls() }

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
  #if os(macOS)
    @Environment(\.openSettings) private var openSettings
  #endif
  var body: some View {
    if let controller = state.controller {
      RecorderScreen(
        controller: controller, input: state.input, state: state, url: url,
        recoveryRevision: recoveryRevision)
        #if os(macOS)
          .onAppear {
            state.requestSettings = { pane, project in
              state.settingsPane = pane
              state.settingsProject = project
              openSettings()
            }
          }
        #endif
    } else {
      Text("The recorder could not connect to the local application server.").padding()
    }
  }
}

#if os(macOS)
  struct ApplicationSettingsView: View {
    @ObservedObject var state: ApplicationState
    var body: some View {
      TabView(selection: $state.settingsPane) {
        RecordingSettings(input: state.input, controller: state.controller)
          .tabItem { Label("Recording", systemImage: "mic") }.tag("recording")
        Group {
          if let origin = state.origin {
            let path = state.settingsProject.isEmpty
              ? "/settings/machines" : "/settings/machines/projects/\(state.settingsProject)"
            if let url = URL(string: path, relativeTo: origin)?.absoluteURL {
              ApplicationWebView(url: url, recoveryRevision: 0, onOpenSession: { path in
                state.workspace.requestProject?(path)
                state.workspace.webView?.window?.makeKeyAndOrderFront(nil)
              })
            }
          } else { Text("The local application server is unavailable.").padding() }
        }
        .tabItem { Label("Machines", systemImage: "desktopcomputer") }.tag("machines")
      }
      .frame(minWidth: 760, minHeight: 560)
    }
  }
#endif
