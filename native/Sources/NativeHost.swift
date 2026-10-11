import Foundation
import SwiftUI

@MainActor final class NativeHost: ObservableObject {
  static weak var active: NativeHost?
  @Published private(set) var url: URL?
  @Published private(set) var failure: String?
  @Published private(set) var recoveryRevision = 0
  // Set by ApplicationState once. The shell retains the same state on resume.
  var onShutdown: (() -> Void)?
  var onBackgroundChange: ((Bool) -> Void)?
  var onAvailabilityChange: ((Bool, String?) -> Void)?
  private var foregroundTask: Task<Void, Never>?
  private var wasBackgrounded = false
  private var stopped = false
  init() {
    Self.active = self
    guard let pointer = SKGoHostStart() else {
      failure = "The local application server could not start."
      return
    }
    let origin = String(cString: pointer)
    free(pointer)
    guard let url = URL(string: origin), url.host == "127.0.0.1" else {
      failure = "Invalid local application origin."
      SKGoHostStop()
      return
    }
    self.url = url
  }
  func shutdown() {
    guard !stopped else { return }
    stopped = true
    foregroundTask?.cancel()
    onShutdown?()
    SKGoHostStop()
  }
  func didEnterBackground() {
    wasBackgrounded = true
    foregroundTask?.cancel()
    onBackgroundChange?(true)
  }
  func didBecomeActive() {
    onBackgroundChange?(false)
    guard wasBackgrounded, !stopped, let url else { return }
    wasBackgrounded = false
    foregroundTask?.cancel()
    foregroundTask = Task {
      let config = URLSessionConfiguration.ephemeral
      config.timeoutIntervalForRequest = 1
      config.timeoutIntervalForResource = 1
      let session = URLSession(configuration: config)
      defer { session.invalidateAndCancel() }
      do {
        let (_, response) = try await session.data(
          from: url.appendingPathComponent("__skgo_health"))
        guard (response as? HTTPURLResponse)?.statusCode == 204 else {
          throw URLError(.badServerResponse)
        }
      } catch {
        guard !Task.isCancelled else { return }
        if SKGoHostRecover() == 0 {
          wasBackgrounded = true
          let message = "The local server could not reconnect. Return to the app to retry."
          failure = message
          onAvailabilityChange?(false, message)
          return
        }
      }
      guard !Task.isCancelled else { return }
      failure = nil
      onAvailabilityChange?(true, nil)
      recoveryRevision += 1
    }
  }
}
