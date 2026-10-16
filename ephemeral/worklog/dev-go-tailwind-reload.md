# Backend edit reloads

correction: A browser draft and workspace tabs disappearing across Go replacement did not establish that SKGO or Gimbal component state reset them. A controlled edit inside web caused Tailwind to send an explicit Vite full-reload before Go publication; the same structural edit outside web and plain Kit refresh preserved state.

decision: Exclude backend Go files with Tailwind's public `@source not` directive. Keep native Go rebuild and workflow-source watchers intact; do not intercept generic Vite reload messages.
