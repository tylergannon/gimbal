import SwiftUI
import WebKit

#if os(macOS)
  import AppKit
#endif

struct RecorderScreen: View {
  @ObservedObject var controller: RecorderController
  @ObservedObject var input: RecordingInput
  let state: ApplicationState
  let url: URL
  let recoveryRevision: Int
  var body: some View {
    #if os(macOS)
      TranscriptWebView(
        controller: controller, state: state, url: url, recoveryRevision: recoveryRevision
      )
      .frame(maxWidth: .infinity, maxHeight: .infinity)
    #else
      VStack(spacing: 0) {
        HStack {
          Text(
            Bundle.main.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String ?? Bundle
              .main
              .object(forInfoDictionaryKey: "CFBundleName") as? String ?? "Recorder"
          ).font(.title2.weight(.bold))
          Spacer()
          if !controller.status.sessionID.isEmpty && controller.canStop
            && controller.status.state != "preparing"
          {
            HStack(spacing: 10) {
              RecordingLevelMeter(controller: controller)
              RecordingTimer(controller: controller)
            }
          }
          #if os(iOS)
            InputDeviceMenu(input: input, enabled: controller.canRecord)
          #endif
        }
        .padding(.horizontal, 24).padding(.vertical, 12)
        if input.fixtureURL == nil {
          RecordingInputNotice(input: input)
            .padding(.horizontal, 24)
        }
        TranscriptWebView(
          controller: controller, state: state, url: url, recoveryRevision: recoveryRevision
        )
      }
      .safeAreaInset(edge: .bottom, spacing: 0) {
        RecorderControls(controller: controller)
          .frame(maxWidth: .infinity)
          .padding(.vertical, 12)
          .background(.bar)
      }
    #endif
  }
}

@MainActor
final class WebCoordinator: NSObject, WKScriptMessageHandler, WKNavigationDelegate, WKUIDelegate {
  let controller: RecorderController
  let state: ApplicationState
  weak var webView: WKWebView?
  var expectedOrigin: URL?
  private var foreground = true
  private var recoveryRevision = 0
  private var signingIn = false
  init(controller: RecorderController, state: ApplicationState) {
    self.controller = controller
    self.state = state
  }
  func userContentController(
    _ userContentController: WKUserContentController, didReceive message: WKScriptMessage
  ) {
    guard message.frameInfo.isMainFrame,
      let origin = expectedOrigin,
      message.frameInfo.securityOrigin.protocol == origin.scheme,
      message.frameInfo.securityOrigin.host == origin.host,
      message.frameInfo.securityOrigin.port == origin.port,
      let body = message.body as? [String: Any]
    else { return }
    #if os(macOS)
      if message.name == "workspaceState", let ready = body["ready"] as? Bool,
        let visible = body["sidebarVisible"] as? Bool,
        let count = body["tabCount"] as? Int
      {
        state.workspace.update(
          ready: ready, sidebarVisible: visible, tabCount: count,
          activeTabTitle: body["activeTabTitle"] as? String,
          canStopResponse: body["canStopResponse"] as? Bool ?? false)
        return
      }
      if message.name == "workspaceDrag", let event = NSApp.currentEvent,
        event.type == .leftMouseDown
      {
        webView?.window?.performDrag(with: event)
        return
      }
    #endif
    if message.name == "selectProject", let project = body["project"] as? String {
      state.commands?.selectedProject = project
      return
    }
    #if os(macOS)
      if message.name == "openFile", let path = body["path"] as? String {
        state.openViewer(path: path, origin: origin)
        return
      }
    #endif
    if message.name == "reconnect" {
      reloadTranscript()
      return
    }
    if message.name == "bridgeReady" {
      send(controller.status)
      return
    }
    if message.name == "snapshotApplied", let id = body["sessionID"] as? String,
      let sequence = body["sequence"] as? Int
    {
      controller.snapshotApplied(sessionID: id, sequence: sequence, closed: body["closed"] as? Bool)
    }
    if message.name == "displayAvailability", let available = body["available"] as? Bool {
      controller.displayAvailability(available, error: body["error"] as? String)
    }
  }
  func webView(
    _ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction,
    decisionHandler: @escaping @MainActor @Sendable (WKNavigationActionPolicy) -> Void
  ) {
    #if os(macOS)
      if navigationAction.targetFrame == nil, let target = navigationAction.request.url,
        ["https", "http"].contains(target.scheme ?? "")
      {
        NSWorkspace.shared.open(target)
        decisionHandler(.cancel)
        return
      }
    #endif
    if navigationAction.targetFrame?.isMainFrame == false {
      decisionHandler(.allow)
      return
    }
    if let target = navigationAction.request.url, let origin = expectedOrigin,
      let server = NativeHost.active?.serverURL
    {
      if signingIn, target.scheme == server.scheme, target.host == server.host,
        target.port == server.port, target.path == "/auth/callback",
        var callback = URLComponents(url: target, resolvingAgainstBaseURL: false)
      {
        callback.scheme = origin.scheme
        callback.host = origin.host
        callback.port = origin.port
        signingIn = false
        decisionHandler(.cancel)
        if let callbackURL = callback.url { webView.load(URLRequest(url: callbackURL)) }
        return
      }
      if target.scheme == "https", target.host == "github.com",
        signingIn || target.path == "/login/oauth/authorize"
      {
        signingIn = true
        decisionHandler(.allow)
        return
      }
    }
    guard let target = navigationAction.request.url, let origin = expectedOrigin,
      target.scheme == origin.scheme, target.host == origin.host, target.port == origin.port
    else {
      decisionHandler(.cancel)
      return
    }
    decisionHandler(.allow)
  }
  func webView(_ webView: WKWebView, didStartProvisionalNavigation navigation: WKNavigation!) {
    #if os(macOS)
      state.workspace.ready = false
    #endif
  }
  func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
    NSLog("Workspace loaded frame=%@", NSStringFromRect(webView.frame))
    send(controller.status)
  }
  func webView(
    _ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!,
    withError error: Error
  ) {
    NSLog("Workspace navigation failed: %@", error.localizedDescription)
  }
  func webView(
    _ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
    for navigationAction: WKNavigationAction, windowFeatures: WKWindowFeatures
  ) -> WKWebView? {
    #if os(macOS)
      if let target = navigationAction.request.url, ["http", "https"].contains(target.scheme ?? "")
      {
        NSWorkspace.shared.open(target)
      }
    #endif
    return nil
  }
  #if os(macOS)
    func webView(
      _ webView: WKWebView, runOpenPanelWith parameters: WKOpenPanelParameters,
      initiatedByFrame frame: WKFrameInfo,
      completionHandler: @escaping @MainActor @Sendable ([URL]?) -> Void
    ) {
      guard let window = webView.window else {
        completionHandler(nil)
        return
      }
      let panel = NSOpenPanel()
      panel.allowsMultipleSelection = parameters.allowsMultipleSelection
      panel.canChooseDirectories = parameters.allowsDirectories
      panel.canChooseFiles = true
      panel.beginSheetModal(for: window) { response in
        completionHandler(response == .OK ? panel.urls : nil)
      }
    }
  #endif
  func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
    controller.displayAvailability(false, error: "Transcript display is reconnecting.")
    if foreground { reloadTranscript() }
  }
  func setForeground(_ value: Bool, recoveryRevision revision: Int) {
    let recovered = revision != recoveryRevision
    recoveryRevision = revision
    foreground = value
    // WebKit may suspend its live query or discard the page while hidden.
    // Reload from the existing native server, never restart the session.
    if value && recovered { reloadTranscript() }
  }
  private func reloadTranscript() {
    guard let origin = expectedOrigin else { return }
    controller.displayAvailability(false, error: "Transcript display is reconnecting.")
    webView?.load(URLRequest(url: webView?.url ?? origin))
  }
  func send(_ status: NativeStatus) {
    guard foreground else { return }
    // Provider pages never receive recording text or native workspace state.
    guard let target = webView?.url, let origin = expectedOrigin,
      target.scheme == origin.scheme, target.host == origin.host, target.port == origin.port
    else { return }
    guard let data = try? JSONEncoder().encode(status),
      var object = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
    else { return }
    object["project"] = state.commands?.recordingProject ?? ""
    guard let data = try? JSONSerialization.data(withJSONObject: object),
      let json = String(data: data, encoding: .utf8)
    else { return }
    webView?.evaluateJavaScript(
      "window.voiceNotesLatestStatus = \(json); window.voiceNotesNativeStatus && window.voiceNotesNativeStatus(window.voiceNotesLatestStatus);",
      completionHandler: nil)
  }
  func make() -> WKWebView {
    let config = WKWebViewConfiguration()
    config.websiteDataStore = .nonPersistent()
    config.userContentController.add(self, name: "snapshotApplied")
    config.userContentController.add(self, name: "displayAvailability")
    config.userContentController.add(self, name: "bridgeReady")
    config.userContentController.add(self, name: "reconnect")
    config.userContentController.add(self, name: "selectProject")
    config.userContentController.add(self, name: "openFile")
    #if os(macOS)
      config.userContentController.add(self, name: "workspaceState")
      config.userContentController.add(self, name: "workspaceDrag")
      config.userContentController.addUserScript(
        WKUserScript(
          source: "window.gimbalNativeWorkspace = true;", injectionTime: .atDocumentStart,
          forMainFrameOnly: true))
      let view = WorkspaceWebView(frame: .zero, configuration: config)
      view.workspace = state.workspace
      state.workspace.webView = view
    #else
      let view = WKWebView(frame: .zero, configuration: config)
    #endif
    view.uiDelegate = self
    view.navigationDelegate = self
    webView = view
    controller.onStatus = { [weak self] status in self?.send(status) }
    return view
  }
}
#if os(macOS)
  struct TranscriptWebView: NSViewRepresentable {
    @ObservedObject var controller: RecorderController
    let state: ApplicationState
    let url: URL
    let recoveryRevision: Int
    func makeCoordinator() -> WebCoordinator {
      WebCoordinator(controller: controller, state: state)
    }
    func makeNSView(context: Context) -> WKWebView {
      context.coordinator.expectedOrigin = url
      let view = context.coordinator.make()
      NSLog("Workspace loading %@", url.absoluteString)
      view.load(URLRequest(url: url))
      return view
    }
    func updateNSView(_ nsView: WKWebView, context: Context) {
      context.coordinator.setForeground(true, recoveryRevision: recoveryRevision)
    }
  }
#else
  struct TranscriptWebView: UIViewRepresentable {
    @ObservedObject var controller: RecorderController
    @Environment(\.scenePhase) private var scenePhase
    let state: ApplicationState
    let url: URL
    let recoveryRevision: Int
    func makeCoordinator() -> WebCoordinator {
      WebCoordinator(controller: controller, state: state)
    }
    func makeUIView(context: Context) -> WKWebView {
      context.coordinator.expectedOrigin = url
      let view = context.coordinator.make()
      view.load(URLRequest(url: url))
      return view
    }
    func updateUIView(_ uiView: WKWebView, context: Context) {
      context.coordinator.setForeground(
        scenePhase != .background, recoveryRevision: recoveryRevision)
    }
  }
#endif
