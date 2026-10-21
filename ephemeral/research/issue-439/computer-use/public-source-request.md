# Public Codex source investigation

User authorizes downloading public ChatGPT/Codex source and tracing exactly which conditions enable or disable computer use for a session, to inform a macOS resident Gimbal worker backend. Repository: https://github.com/openai/codex . Download source into a new temporary directory outside the Gimbal repository; record the exact revision and location. Do not build or execute downloaded code.

Trace tool exposure, plugin enablement, app-server/session metadata, remote/SSH handling, and any process-origin or native computer-use authorization checks present in this repository. Separate found conditions from desktop-only/private pieces absent from public source. Cite exact source file/line and stable GitHub commit links. Do not infer permission from a name or configuration alone.

Known local incident: earlier today an SSH-origin task could list apps but failed Finder capture while locked with cgWindowNotFound; desktop logs reportedly said untrusted-process-ancestry. Local desktop-origin capture and Locked Use succeeded. Those observations do not prove launchd solves it. Another registration/UI issue made setup toggles disabled while the installed plugin was enabled.

You own only the public-source investigation and public-source-report.md beside this file. Another agent is examining installed desktop code; do not duplicate that work. You are not alone in this codebase: do not revert others' edits. Do not change permissions, running apps, services, account configuration, or secrets. No bypasses or patched authorization checks. Return a source-grounded report and exact limitations. Do not commit; parent handles documentation commits.
