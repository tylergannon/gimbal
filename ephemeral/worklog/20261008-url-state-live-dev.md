# URL state and live development

decision: Keep source-only review in internal/generate/flowview, with a thin Kit 3 dev route around the shared portable renderer. Query codec owns exact-key intersection/defaults; event handlers commit shallow URLs without effect synchronization.
correction: Shallow browser history in Kit 3.0.1 skips afterNavigate. Use an explicit same-route popstate handler reading location.href; do not trust an API summary instead of browser/source verification.
correction: Preserve hidden child disclosure paths without adding ancestors, so parent collapse remains closed.
friction: Vite configs/tests sharing optimize cache corrupted live dependency URLs -> independent Kit/portable/test caches.
decision: Source watcher serializes/debounces extraction, retains last-good data on errors, refreshes local input closure and retains watches through invalid source. External updates are custom events over Vite's transport. The frozen portable checkpoint and runtime app are outside this iteration.
