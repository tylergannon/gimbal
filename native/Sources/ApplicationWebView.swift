import SwiftUI
import WebKit

#if os(macOS)
  import AppKit
#endif

@MainActor final class ApplicationWebCoordinator: NSObject, WKNavigationDelegate, WKUIDelegate, WKScriptMessageHandler {
  var url: URL?
  var revision = 0
  private var signingIn = false
  var onOpenSession: ((String) -> Void)?
  func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage) {
    guard message.frameInfo.isMainFrame, let url, url.path.hasPrefix("/settings/"),
      message.frameInfo.securityOrigin.protocol == url.scheme,
      message.frameInfo.securityOrigin.host == url.host,
      message.frameInfo.securityOrigin.port == url.port,
      let body = message.body as? [String: Any] else { return }
    #if os(macOS)
      if message.name == "openMachineTerminal", body["local"] as? Bool == true,
        let terminal = NSWorkspace.shared.urlForApplication(withBundleIdentifier: "com.apple.Terminal") {
        NSWorkspace.shared.openApplication(at: terminal, configuration: NSWorkspace.OpenConfiguration())
        return
      }
      if message.name == "openMachineTerminal", let host = body["host"] as? String,
        !host.isEmpty, !host.hasPrefix("-"), !host.contains(where: { $0.isWhitespace }) {
        var components = URLComponents()
        components.scheme = "ssh"
        let parts = host.split(separator: "@", maxSplits: 1)
        if parts.count == 2 { components.user = String(parts[0]) }
        components.host = String(parts.last ?? "")
        if let port = body["port"] as? Int, port > 0 { components.port = port }
        if let target = components.url { NSWorkspace.shared.open(target) }
      }
      if message.name == "openSettingsSession", let path = body["path"] as? String,
        path.hasPrefix("/projects/"), !path.contains(".."), !path.contains("?") {
        onOpenSession?(path)
      }
    #endif
  }
  func update(_ view: WKWebView, url: URL, revision: Int) {
    if self.url != url || self.revision != revision {
      self.url = url
      self.revision = revision
      view.load(URLRequest(url: url))
    }
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
  func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
    if let url { webView.load(URLRequest(url: url)) }
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
    if let target = navigationAction.request.url, let url,
      let server = NativeHost.active?.serverURL
    {
      if signingIn, target.scheme == server.scheme, target.host == server.host,
        target.port == server.port, target.path == "/auth/callback",
        var callback = URLComponents(url: target, resolvingAgainstBaseURL: false)
      {
        // Keep the flow cookie and the new session inside the native proxy.
        // GitHub still uses the server's registered callback, unchanged.
        callback.scheme = url.scheme
        callback.host = url.host
        callback.port = url.port
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
    guard let target = navigationAction.request.url, let url,
      target.scheme == url.scheme, target.host == url.host, target.port == url.port
    else {
      decisionHandler(.cancel)
      return
    }
    decisionHandler(.allow)
  }
  func make() -> WKWebView {
    let config = WKWebViewConfiguration()
    config.websiteDataStore = .nonPersistent()
    config.userContentController.add(self, name: "openMachineTerminal")
    config.userContentController.add(self, name: "openSettingsSession")
    let view = WKWebView(frame: .zero, configuration: config)
    view.uiDelegate = self
    view.navigationDelegate = self
    return view
  }
}
#if os(macOS)
  struct ApplicationWebView: NSViewRepresentable {
    let url: URL
    let recoveryRevision: Int
    var onOpenSession: ((String) -> Void)? = nil
    func makeCoordinator() -> ApplicationWebCoordinator { ApplicationWebCoordinator() }
    func makeNSView(context: Context) -> WKWebView { context.coordinator.make() }
    func updateNSView(_ view: WKWebView, context: Context) {
      context.coordinator.onOpenSession = onOpenSession
      context.coordinator.update(view, url: url, revision: recoveryRevision)
    }
  }
#else
  struct ApplicationWebView: UIViewRepresentable {
    let url: URL
    let recoveryRevision: Int
    func makeCoordinator() -> ApplicationWebCoordinator { ApplicationWebCoordinator() }
    func makeUIView(context: Context) -> WKWebView { context.coordinator.make() }
    func updateUIView(_ view: WKWebView, context: Context) {
      context.coordinator.update(view, url: url, revision: recoveryRevision)
    }
  }
#endif
