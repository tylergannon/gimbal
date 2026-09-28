# Assignment: prepare the consumer image and characterize browser tooling

User authorizes fifteen claims in ephemeral/browser-evaluator-build.md. You
own preparation of the Docker evaluation image, entirely outside the tracked
tree under /tmp/gimbal-browser-build.AfTh8P/image. You are not alone; do not
revert others or edit tracked files. Do not commit, invoke other agents or Gimbal.

Docker Desktop is running on this arm64 Mac. Prepare a reasonably small Linux
image containing Node, Codex CLI 0.157.1 (current prototype harness), zsh, git,
procps, curl, ffmpeg with libx264, and working playwright-cli with a headless
Chromium browser. Current host playwright-cli is 0.1.21. Do NOT include a Gimbal
binary or build Gimbal into the image: the backend will inject it at runtime.
Prefer an existing Playwright browser base if it avoids slow dependency setup.
Pin versions actually used. Put Dockerfile and a short README in your external
directory, build tag gimbal-browser-evaluator:local, and use real browser open,
navigation, screenshot, recording stop/close, and ffmpeg checks to establish
the CLI/config shape. Do not call models or inspect/login/copy credentials.

The target app is prepared TodoMVC JavaScript ES6 at
/tmp/gimbal-browser-build.AfTh8P/todomvc/examples/javascript-es6/dist
(upstream commit ff43b02e59dfa604386bb382034b2cd07c2bcd8a).
Use a foreground server in your owned probe container and exercise it over
loopback. The image should serve already-built app assets without needing npm
installation per agent session. No root daemon/browser process should remain
after your probe container is removed. Keep the successful image for the manager.

Return a concise report with image ID/architecture, exact CLI/config/recording
commands, artifact paths, and any runtime limitations. Record primary-source
URLs when consulting docs. Don't build a generalized image manager or test harness.
The scope-bound browser API designer is working separately; your findings should
inform it without prescribing new public interfaces.
