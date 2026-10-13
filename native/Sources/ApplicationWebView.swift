import SwiftUI
import WebKit

#if os(macOS)
  import AppKit
#endif

@MainActor final class ApplicationWebCoordinator: NSObject, WKNavigationDelegate, WKUIDelegate {
  var url: URL?
  var revision = 0
  private var signingIn = false
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
    func makeCoordinator() -> ApplicationWebCoordinator { ApplicationWebCoordinator() }
    func makeNSView(context: Context) -> WKWebView { context.coordinator.make() }
    func updateNSView(_ view: WKWebView, context: Context) {
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
