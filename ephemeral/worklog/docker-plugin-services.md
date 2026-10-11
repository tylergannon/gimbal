# Docker plugin services

assessment: Main b2264abe contains PR444 task execution, PR446 native workflow/backend plugins and independent A/B execution, and PR447 follow reconnection. Their recorded live evidence covers the previously proposed first stride. Do not rebuild those capabilities.
decision: Finish the bundled Docker provisioner's concrete integration gap: configured native plugin directories and workspace settings reach its daemon, and explicit service networking lets a workflow consume a scope-owned service through the registered Docker backend. Retain receipt-owned stop and existing event/control contracts.
scope: Configure existing daemon/network capabilities, validate their Docker-specific inputs, document actual plugin-compatible builds, and demonstrate external workflow launch, service use, and scope release. No new backend family, message bus, resource provisioning system, or daemon-crash recovery.
proof: Use the actual built CLI and Docker plugin/image to launch an external workflow, retrieve the service response through its returned URL, observe completion and output through central, and confirm service cleanup. This change needs no provider turn because provider execution is unchanged.

correction: Extending ownership receipts requires extending the registered backend JSON schema too. Exercise Start receipt through Registry.Connect; unmarshalling alone misses schema rejection.
friction: Linux native plugin builds require CGO and a matching Linux compiler for both host binary and plugins; a CGO-disabled task image cannot demonstrate plugin loading.
observation: An A/B workflow failed on a macOS Docker Desktop bind-mounted workflow directory but completed with a Docker volume, with the same binary/plugins. Document the working volume setup and investigate the lifetime-lock boundary separately. Central and remote workflow state must remain separate.
follow-up: Issue #448 records the observed bind-mount failure without asserting a lock root cause. Issue #449 records the independently traced project-list failure after rejected admission; do not claim that UI path passed.
