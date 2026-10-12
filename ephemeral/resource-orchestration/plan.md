# Resource orchestration: #466 and #467 together

Planning base: PR #468 head 2f94af240339eef5c6a9f2fb277b2eb2b1042e1d. Product implementation waits for its actual squash merge, then moves onto origin/main without replaying foundation commits.

## Consumer and desired behavior

Replace service-check's shared-workdir service-address carrier with one named HTTP service, `http`, acquired for its owning scope and used by its actual agent and command executor. Workflow source names the service and consumer, with ordinary Go retaining progression. External deployment configuration chooses placement, consumer-relative URL, and readiness path/status; the workflow contains no topology or concrete resource access. The initial service is externally provisioned and shared: acquisition borrows ready access, release relinquishes the scope's use, and never stops infrastructure it does not own. Native agent and process ownership remains in the task daemon.

This deliberately settles a small first contract: availability is checked at acquisition and actual use; later loss is an ordinary consumer failure, with no continuous health, restart, or failover promise. Process launch is not acquisition or readiness. Failed readiness or executor access must prevent reporting acquired access. A workflow receives a logical binding carrier, not an address it must redistribute.

## Smallest changes

- A service-only orchestration interface takes named service, backend.Scope ownership, and logical consumer, and returns a non-secret HTTP binding. It releases one scope's leases. Final names/signatures follow actual call sites; no general resource vocabulary.
- A concrete configured HTTP-service implementation is constructed outside workflow code. Runtime construction accepts that interface explicitly and supplies it to messaging connection/worker construction. Each process constructs its own implementation from construction data, including the separate runner and taskConnection used by exited-run cleanup. Empty configuration means absent dependency, preserving ordinary existing deployments.
- Construction data is separate from transport configuration and persists with the existing task-route ownership record. No Go pointer crosses JSON. Connection close releases connection resources, not scope leases or native tasks. Scope cleanup and exited cleanup explicitly release use with the same ownership identity.
- Service-check acquires the logical service for both supported consumer kinds. Process requests carry non-secret bindings; the task daemon converts the selected binding to a command environment variable. Turn requests carry the binding; the actual agent receives the HTTP URL in runtime context. Binding resolution uses the actual agent/process producer identity, not one route label.
- HTTP messaging's localhost-only constraint needs a bounded explicitly configured remote endpoint allowance for the Docker consumer. No discovery or transport framework. Other messaging plugins need only signature updates and nil-dependency coverage.
- Resource-use observations contain service/consumer/scope and readiness/release facts, never concrete implementation handles. No unrelated UI redesign.

## Observable proof

Run service-check through runtime-loaded messaging and its separate runner, with gpt-5.6-luna. Use an actual task producer in a Docker network and a real HTTP service in another container/environment. The native agent must fetch the response and the command must fetch it using the delivered binding, with no address file. Change the service placement/configured consumer-relative URL and repeat the same workflow source. Two simultaneous scope uses establish that releasing one preserves the other's response and the shared service. Failed declared readiness and failed executor access must return acquisition errors. Kill an independent runner while its service-use scope is active, invoke the existing exited-run cleanup path, and observe only that run's leases released.

Focused tests belong beside interface construction, message carriers/executor handling, scope release, and runner cleanup. Required checks are just vet, just test, and build/generation as affected; no committed proof scripts, run output, screenshots, or credentials. Independent validation inspects exact implementation and live proof before a focused unmerged PR.

## Contract boundary needing adjudication

The issues allow shared externally owned infrastructure and do not prescribe a provisioning mechanism. This plan chooses borrowed externally provisioned HTTP services as the first concrete resource implementation, not Docker lifecycle management. If #467 instead requires Gimbal to launch new service machines itself, that is a material ownership choice: the existing service command contract cannot name a service image or placement without putting concrete deployment data in workflow Go. Confirm this consumer boundary before product implementation; do not silently claim borrowed readiness as provisioning.
