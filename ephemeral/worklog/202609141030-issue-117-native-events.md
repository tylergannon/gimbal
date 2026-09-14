decision: Preserve the normalized session event vocabulary; native harness errors project through session.retry.scheduled or session.step.failed, and approval callbacks through permission.asked and permission.replied.
decision: A native willRetry notification keeps the Codex turn alive because the app-server owns the retry; no retry timing or attempt is invented when the callback does not state one.
decision: Completed permission decisions remain projection history so the browser can show what was requested and how the fixed noninteractive policy answered.
