import Foundation

#if os(macOS)
  import AppKit
  import SwiftUI
  import UniformTypeIdentifiers
#endif

enum Diagnostics {
  static var applicationInfo: String {
    let bundle = Bundle.main
    return
      "version=\(bundle.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "unknown") build=\(bundle.object(forInfoDictionaryKey: "CFBundleVersion") as? String ?? "unknown") system=\(ProcessInfo.processInfo.operatingSystemVersionString) app=\(bundle.bundleURL.path)"
  }
  static func record(_ component: String, _ message: String) {
    NSLog("[%@] %@", component, message)
    component.withCString { component in
      message.withCString { message in SKGoHostDiagnostic(component, message) }
    }
  }

  #if os(macOS)
    @MainActor static func export() {
      let panel = NSSavePanel()
      panel.title = "Export Gimbal Diagnostics"
      panel.message =
        "Save recent app and server diagnostic logs. Conversation and recording files are not included."
      panel.allowedContentTypes = [.plainText]
      panel.nameFieldStringValue =
        "Gimbal-diagnostics-\(Date.now.formatted(.iso8601.year().month().day().dateSeparator(.dash))).log"
      panel.canCreateDirectories = true
      guard panel.runModal() == .OK, let url = panel.url else { return }
      record("app", applicationInfo)
      guard let error = url.path.withCString({ SKGoHostExportDiagnostics($0) }) else { return }
      defer { free(error) }
      showError("Could not export diagnostics", String(cString: error))
    }

    @MainActor static func openLogs() {
      guard let path = SKGoHostLogsDirectory() else {
        showError("Could not open logs", "The Gimbal logs folder is unavailable.")
        return
      }
      defer { free(path) }
      if !NSWorkspace.shared.open(URL(fileURLWithPath: String(cString: path), isDirectory: true)) {
        showError("Could not open logs", "Finder could not open the Gimbal logs folder.")
      }
    }

    @MainActor private static func showError(_ title: String, _ message: String) {
      let alert = NSAlert()
      alert.messageText = title
      alert.informativeText = message
      alert.alertStyle = .warning
      alert.runModal()
    }
  #endif
}

#if os(macOS)
  struct DiagnosticCommands: Commands {
    var body: some Commands {
      CommandGroup(after: .help) {
        Button("Export Diagnostics…", action: Diagnostics.export)
        Button("Open Logs Folder", action: Diagnostics.openLogs)
      }
    }
  }
#endif
