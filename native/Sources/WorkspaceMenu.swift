#if os(macOS)
  import AppKit
  import SwiftUI
  import WebKit

  @MainActor final class WorkspaceState: ObservableObject {
    @Published var ready = false
    @Published var sidebarVisible = false
    @Published var tabCount = 0
    @Published var hasActiveTab = false
    @Published var windowIsKey = false
    @Published var canStopResponse = false
    weak var webView: WKWebView?

    var enabled: Bool { ready && windowIsKey }

    func update(ready: Bool, sidebarVisible: Bool, tabCount: Int, activeTabTitle: String?, canStopResponse: Bool = false) {
      self.sidebarVisible = sidebarVisible
      self.tabCount = tabCount
      hasActiveTab = activeTabTitle != nil
      self.canStopResponse = canStopResponse
      self.ready = ready
    }

    func send(_ command: String) {
      guard enabled else { return }
      webView?.evaluateJavaScript(
        "window.dispatchEvent(new CustomEvent('gimbal-workspace-command', {detail: {command: '\(command)'}}));",
        completionHandler: nil)
    }
  }

  struct WorkspaceMenu: Commands {
    @ObservedObject var workspace: WorkspaceState

    var body: some Commands {
      CommandMenu("Project") {
        Toggle(
          "Show Sidebar",
          isOn: Binding(
            get: { workspace.sidebarVisible },
            set: { _ in workspace.send("toggleSidebar") })
        )
        .keyboardShortcut("s", modifiers: [.command, .control])
        .disabled(!workspace.enabled)
      }
      CommandMenu("Chat") {
        Button("Stop Response") { workspace.send("stopResponse") }
          .keyboardShortcut(".", modifiers: [.command])
          .disabled(!workspace.enabled || !workspace.canStopResponse)
        Divider()
        Button("Close Tab") { workspace.send("closeTab") }
          .keyboardShortcut("w", modifiers: [.command])
          .disabled(!workspace.enabled || !workspace.hasActiveTab)
        Divider()
        Button("Previous Tab") { workspace.send("previousTab") }
          .keyboardShortcut(.tab, modifiers: [.control, .shift])
          .disabled(!workspace.enabled || workspace.tabCount < 2)
        Button("Next Tab") { workspace.send("nextTab") }
          .keyboardShortcut(.tab, modifiers: [.control])
          .disabled(!workspace.enabled || workspace.tabCount < 2)
      }
    }
  }

  // Only the two reserved blank ends of the title strip move the native window.
  // Web controls keep their ordinary mouse handling, including tab buttons.
  final class WorkspaceWebView: WKWebView {
    weak var workspace: WorkspaceState?
    private var keyObserver: NSObjectProtocol?
    private var resignObserver: NSObjectProtocol?

    override func viewDidMoveToWindow() {
      super.viewDidMoveToWindow()
      if let keyObserver { NotificationCenter.default.removeObserver(keyObserver) }
      if let resignObserver { NotificationCenter.default.removeObserver(resignObserver) }
      guard let window else { return }
      window.titleVisibility = .hidden
      window.titlebarAppearsTransparent = true
      window.styleMask.insert(.fullSizeContentView)
      window.isMovableByWindowBackground = false
      window.contentMinSize = NSSize(width: 800, height: 500)
      if window.frameAutosaveName.isEmpty { window.setFrameAutosaveName("GimbalWorkspace") }
      workspace?.windowIsKey = window.isKeyWindow
      keyObserver = NotificationCenter.default.addObserver(
        forName: NSWindow.didBecomeKeyNotification, object: window, queue: .main
      ) { [weak self] _ in
        MainActor.assumeIsolated { self?.workspace?.windowIsKey = true }
      }
      resignObserver = NotificationCenter.default.addObserver(
        forName: NSWindow.didResignKeyNotification, object: window, queue: .main
      ) { [weak self] _ in
        MainActor.assumeIsolated { self?.workspace?.windowIsKey = false }
      }
    }

    override func performKeyEquivalent(with event: NSEvent) -> Bool {
      if workspace?.enabled == true, (workspace?.tabCount ?? 0) >= 2,
        event.keyCode == 0x30, event.modifierFlags.contains(.control),
        !event.modifierFlags.contains(.command), !event.modifierFlags.contains(.option)
      {
        workspace?.send(event.modifierFlags.contains(.shift) ? "previousTab" : "nextTab")
        return true
      }
      // Cmd-W belongs to the active workspace tab, including its empty state.
      // Do not let the standard File / Close Window command stop this window.
      if workspace?.enabled == true,
        event.modifierFlags.intersection(.deviceIndependentFlagsMask) == .command,
        event.charactersIgnoringModifiers == "w"
      {
        if workspace?.hasActiveTab == true { workspace?.send("closeTab") }
        return true
      }
      return super.performKeyEquivalent(with: event)
    }

    private func isDragRegion(_ point: NSPoint) -> Bool {
      let distanceFromTop = isFlipped ? point.y : bounds.height - point.y
      return workspace?.ready == true && bounds.contains(point) && distanceFromTop < 40
        && (point.x < 78 || point.x > bounds.width - 72)
    }

    override func hitTest(_ point: NSPoint) -> NSView? {
      let local = convert(point, from: superview)
      if isDragRegion(local) { return self }
      return super.hitTest(point)
    }

    override func mouseDown(with event: NSEvent) {
      if isDragRegion(convert(event.locationInWindow, from: nil)) {
        window?.performDrag(with: event)
      } else {
        super.mouseDown(with: event)
      }
    }
  }
#endif
