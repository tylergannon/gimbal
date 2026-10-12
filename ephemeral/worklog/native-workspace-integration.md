# Native workspace integration

correction: Previous slice delivered Storybook components, not the production macOS app. This slice must prove the actual ApplicationView → RecorderScreen → main WKWebView with real project content and genuine macOS system menus.

decision: Extend the existing main WebCoordinator with explicit workspace state and four view commands. Preserve RecorderController and RecordingMenu ownership; removing the redundant native top/bottom bars leaves the web shell as the main window content.

friction: WKWebView's asynchronous script messages cannot reliably supply the original mouse-down event for NSWindow.performDrag. The reserved blank ends of the native title strip also intercept mouse-down directly, while tab buttons retain ordinary handling.

friction: Native proof setup initially encountered a locked Mac before Finder screenshot/action could be demonstrated. Continue source checks and build; require unlock before native behavioral claims or screenshots.
