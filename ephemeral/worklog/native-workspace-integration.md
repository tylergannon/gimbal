# Native workspace integration

correction: Previous slice delivered Storybook components, not the production macOS app. This slice must prove the actual ApplicationView → RecorderScreen → main WKWebView with real project content and genuine macOS system menus.

decision: Extend the existing main WebCoordinator with explicit workspace state and four view commands. Preserve RecorderController and RecordingMenu ownership; removing the redundant native top/bottom bars leaves the web shell as the main window content.

friction: WKWebView's asynchronous script messages cannot reliably supply the original mouse-down event for NSWindow.performDrag. The reserved blank ends of the native title strip also intercept mouse-down directly, while tab buttons retain ordinary handling.

friction: CUA initially reported a locked Mac; restarting its stale helper and rebinding recovered native access without a setting change or manual unlock. Establish Finder screenshot/action before inferring anything about Gimbal. Later physical-input detection interrupted native proof again.

friction: CUA captures regular native windows but reports Screenshot unavailable while a macOS menu is open. Record menu behavior and validation state from direct native AX observations, and capture the resulting workspace state; never substitute browser menu images.

friction: Actual native create-conversation navigation found a blank view despite successful server creation. A stale cached inventory won over newly loaded route data, so the selected chat never entered the retained tab list. Runtime proof must include create → open, not just reopening an existing chat.

decision: Declare SwiftUI hiddenTitleBar at the Scene level as well as full-size native content. Merely setting transparent/hidden NSWindow title properties left a separate 32-point blank title row above the web tab strip in the actual window.
