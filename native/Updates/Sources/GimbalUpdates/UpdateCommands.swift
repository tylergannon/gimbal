#if os(macOS)
  import Sparkle
  import SwiftUI

  @MainActor
  private final class Updater: ObservableObject {
    let controller = SPUStandardUpdaterController(
      startingUpdater: true, updaterDelegate: nil, userDriverDelegate: nil)
    @Published var canCheckForUpdates = false

    init() {
      controller.updater.publisher(for: \.canCheckForUpdates)
        .assign(to: &$canCheckForUpdates)
    }
  }

  public struct GimbalUpdateCommands: Commands {
    @StateObject private var updater = Updater()

    public init() {}

    public var body: some Commands {
      CommandGroup(after: .appInfo) {
        Button("Check for Updates…") { updater.controller.checkForUpdates(nil) }
          .disabled(!updater.canCheckForUpdates)
      }
    }
  }
#endif
