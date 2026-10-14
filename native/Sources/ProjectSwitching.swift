import Foundation

#if os(macOS)
  import AppKit
  import Foundation

  enum RecordingSwitchAction: String, CaseIterable {
    case ask, withoutRecording, andRecord
    var title: String {
      switch self {
      case .ask: "Ask each time"
      case .withoutRecording: "Switch without recording"
      case .andRecord: "Switch and record"
      }
    }
    static let preferenceKey = "recordingProjectSwitchAction"
    static var remembered: Self {
      Self(rawValue: UserDefaults.standard.string(forKey: preferenceKey) ?? "") ?? .ask
    }
  }

  // One desktop operation owns the sheet, save, navigation receipt and new start.
  @MainActor final class ProjectSwitching: NSObject {
    let controller: RecorderController
    let commands: GeneratedCommands
    let workspace: WorkspaceState
    private var operation: Task<Void, Never>?
    private var destination = ""
    private var navigation: CheckedContinuation<Void, Error>?
    private var navigationTimeout: Task<Void, Never>?

    init(controller: RecorderController, commands: GeneratedCommands, workspace: WorkspaceState) {
      self.controller = controller
      self.commands = commands
      self.workspace = workspace
    }

    func request(path: String) {
      guard let project = Self.project(in: path), path.hasPrefix("/projects/"),
        operation == nil else { return }
      if project == commands.selectedProject {
        if path != "/projects/\(project)/recordings" { workspace.navigate(path) }
        return
      }
      let needsSaving = controller.canStop
      guard needsSaving ? (["recording", "paused", "stopped"].contains(controller.status.state) || controller.canRetrySave) : controller.canRecord else {
        controller.showProjectError("Finish the recording operation before switching projects.")
        return
      }
      destination = project
      controller.projectSwitchPending = true
      operation = Task {
        defer {
          destination = ""
          controller.projectSwitchPending = false
          operation = nil
        }
        do {
          var action = RecordingSwitchAction.withoutRecording
          var remember = false
          var requirements = false
          if needsSaving {
            requirements = try await commands.requirementsMode(sessionID: controller.status.sessionID)
            action = .remembered
            if action == .ask {
              let choice = await ask(destination: project, requirements: requirements)
              guard let choice else { return }
              action = choice.0
              remember = choice.1
            }
            try await controller.stopAndSave()
          }
          // Selection is committed only by a completed destination navigation.
          try await withCheckedThrowingContinuation { continuation in
            navigation = continuation
            navigationTimeout = Task { [weak self] in
              try? await Task.sleep(for: .seconds(20))
              guard !Task.isCancelled else { return }
              self?.completeNavigation(project: project, error: "The destination workspace did not become ready. Retry switching.")
            }
            workspace.navigate(path)
          }
          if needsSaving && action == .andRecord {
            // The bridge receipt is sent after destination DOM and observation are ready.
            try await controller.awaitDisplay()
            controller.projectSwitchPending = false
            try await controller.recordAndWait(requirements: requirements)
          }
          if remember { UserDefaults.standard.set(action.rawValue, forKey: RecordingSwitchAction.preferenceKey) }
        } catch { controller.showProjectError(error.localizedDescription) }
      }
    }

    func completeNavigation(project: String, error: String?) {
      guard project == destination, let navigation else { return }
      self.navigation = nil
      navigationTimeout?.cancel()
      if let error {
        if workspace.currentProject == project { commands.selectedProject = project }
        navigation.resume(throwing: RecorderError.message(error))
      } else {
        commands.selectedProject = project
        navigation.resume()
      }
    }

    private func ask(destination: String, requirements: Bool) async -> (RecordingSwitchAction, Bool)? {
      guard let window = workspace.webView?.window else { return nil }
      window.deminiaturize(nil)
      window.makeKeyAndOrderFront(nil)
      NSApp.activate(ignoringOtherApps: true)
      let old = workspace.projects.first { $0.id == commands.recordingProject }?.name ?? "current project"
      let next = workspace.projects.first { $0.id == destination }?.name ?? "new project"
      let alert = NSAlert()
      alert.messageText = "Save the recording in \(old) and switch to \(next)?"
      alert.informativeText = "The \(controller.status.state == "paused" ? "paused " : "")recording in \(old) (\(recordingTime(controller.recordingDuration.elapsed()))) will be saved with that project. Switch and record starts a new \(controller.status.source) recording in \(next).\n\(requirements ? "Requirements discussion · feeds Promises" : "Ordinary recording")"
      alert.addButton(withTitle: "Cancel")
      alert.addButton(withTitle: RecordingSwitchAction.withoutRecording.title)
      alert.addButton(withTitle: RecordingSwitchAction.andRecord.title)
      alert.buttons[0].keyEquivalent = "\u{1b}"
      alert.buttons[2].keyEquivalent = "\r"
      alert.showsSuppressionButton = true
      alert.suppressionButton?.title = "Do this without asking"
      let response = await alert.beginSheetModal(for: window)
      let action: RecordingSwitchAction
      switch response {
      case .alertSecondButtonReturn: action = .withoutRecording
      case .alertThirdButtonReturn: action = .andRecord
      default: return nil
      }
      return (action, alert.suppressionButton?.state == .on)
    }

    func showMenu(x: Double? = nil, y: Double? = nil) {
      guard operation == nil, let view = workspace.webView, let window = view.window else { return }
      window.deminiaturize(nil)
      window.makeKeyAndOrderFront(nil)
      NSApp.activate(ignoringOtherApps: true)
      let menu = NSMenu(title: "Projects")
      for project in workspace.projects {
        let ownsRecording = project.id == commands.recordingProject && controller.canStop
        let marker = controller.status.state == "paused" ? "◌" : "●"
        let title = ownsRecording ? "\(project.name)    \(marker) \(recordingTime(controller.recordingDuration.elapsed()))" : project.name
        let item = NSMenuItem(title: title, action: #selector(selectProject(_:)), keyEquivalent: "")
        item.target = self
        item.representedObject = project.id
        item.state = project.id == commands.selectedProject ? .on : .off
        item.toolTip = project.path
        // Directory paths disambiguate duplicate names without inventing catalog facts.
        if workspace.projects.filter({ $0.name == project.name }).count > 1 {
          item.title += "    \(project.path)"
        }
        menu.addItem(item)
      }
      menu.addItem(.separator())
      let folder = NSMenuItem(title: "Open folder…", action: #selector(openFolder), keyEquivalent: "")
      folder.target = self
      menu.addItem(folder)
      menu.popUp(positioning: nil, at: NSPoint(x: x ?? 100, y: view.bounds.height - (y ?? 50)), in: view)
    }
    @objc private func selectProject(_ item: NSMenuItem) {
      if let project = item.representedObject as? String { request(path: "/projects/\(project)/recordings") }
    }
    @objc private func openFolder() { workspace.send("openProjectFolder") }
    static func project(in path: String) -> String? {
      let parts = path.split(separator: "/")
      return parts.count >= 2 && parts[0] == "projects" ? String(parts[1]) : nil
    }
  }
#endif

func recordingTime(_ elapsed: TimeInterval) -> String {
  let seconds = Int(elapsed)
  return seconds >= 3600
    ? String(format: "%d:%02d:%02d", seconds / 3600, (seconds / 60) % 60, seconds % 60)
    : String(format: "%02d:%02d", seconds / 60, seconds % 60)
}
