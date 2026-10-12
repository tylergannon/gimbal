# Messaging dependency and consumer-binding exercise: #466 and #467

Implementation base: squash-merged main 3deaf2944f896379cbd166a3a22d6361646935d2. Only this batch's planning commits were rebased from PR #468's head.

## Authoritative boundary

The user explicitly instructed this exercise to make **no effort to build the infrastructure backend**. Existing or externally supplied resources are the fixture. This slice exposes the construction, consumer-binding, and scope-release requirements that inform a later orchestration implementation. No provisioning, Docker/machine management, production resource backend, lifecycle subsystem, or lease framework belongs here. The earlier borrowed-versus-provisioned question was an incorrect scope expansion and is withdrawn.

## Desired behavior and concrete consumer

Replace service-check's shared-workdir service-address carrier with a logical named HTTP-service use in its owning scope. The actual agent and command each fetch the supplied service response. Workflow Go names the service and logical consumer and retains ordinary progression; deployment addresses and concrete orchestration types stay outside it. HTTP messaging selects different producers for agents and processes, so one task-route label cannot substitute for identifying the actual executor.

The narrow service interface needs to establish consumer-relative access for a named service and backend.Scope owner, return a non-secret binding, and release that scope's use. Exact signatures follow these call sites, with no general resource vocabulary. The executor receives the binding in its native form: command environment for a command; runtime-supplied context for an agent. Workflow code never redistributes a URL or obtains a concrete implementation by assertion.

## Authorized runtime changes

- Accept an explicitly supplied orchestration interface at messaging/runtime construction. Supply it to the actual messaging factory/worker and service consumer path. JSON describes how the runtime constructs its local dependency; it never transports pointers. Finalize the smallest explicit constructor seam using the exercise fixture, without a DI registry, service locator, or production resource implementation.
- Thread the construction data and interface ownership through the separate runner, normal scope cleanup, and taskConnection used by exited-run cleanup. Reconstruct only process-local dependencies. Connection close closes connection resources; scope release is an explicit operation with run/scope identity. Empty construction configuration preserves ordinary deployments without artificial resources.
- Carry logical service-use information to the selected producer, resolve through its supplied dependency for the actual supported executor, and deliver non-secret bindings to native command/agent execution. Native session/process ownership remains in the existing task daemon.
- Establish declared readiness and consumer access separately from process launch. Return errors for failed declared readiness or access; never report acquisition merely because an address exists. Resource-use observations expose named service/consumer/scope and observed outcome, not implementation handles.
- Update only the concrete messaging endpoint restriction that prevents the exercised remote producer, if needed. No discovery or generalized networking layer. Other transport changes are limited to explicit construction signatures and absent-dependency handling.

## Fixture, implementation proof, and learned requirements

**Fixture supplies:** externally running HTTP services, two placements, executor network environments, externally constructed interface implementations, a declared readiness check, and release behavior that relinquishes one scope's use while retaining shared service availability. Test fixtures belong beside affected tests. Live fixture code/configuration stays outside Git; no committed proof programs.

**The real runtime demonstrates:** runtime-loaded messaging receives the constructed interface; the separate runner reconstructs its own dependency; the selected agent and command actually receive and use the bindings from distinct network environments; unchanged workflow source works under a second placement; normal scope cleanup and existing exited-run cleanup call the supplied release operation with correct ownership; failed readiness/access does not appear acquired; another scope's use stays functional.

**The later infrastructure backend learns:** it must resolve service + scope + logical consumer to actual executor connectivity, establish its declared readiness, deliver usable non-secret bindings, define acquisition/release ownership, preserve unrelated/shared use, and declare supported availability/loss semantics. This slice does not prove provisioning, machine placement automation, backend crash recovery, durable leases, continuous health, restart, or failover.

## Observable proof and gates

Run the actual service-check workflow through runtime-loaded messaging and a separate runner with gpt-5.6-luna. Fixture service and actual remote consumer occupy distinct network environments. Require both agent fetch and command fetch; receiving/displaying a URL is insufficient. Change external placement/configuration and repeat the same workflow source with no shared-workdir address file. Exercise overlapping scopes and show one release preserves the other's response. Exercise readiness/access failure and the existing exited-run cleanup connection path.

Focused tests belong beside construction, carriers/executor handling, scope release, and runner cleanup. Run required checks appropriate to affected code (just vet, just test, build/generation). Independent validation reviews exact code and what the observed runs establish. Commit/push a focused PR for independent validation and coordinating-agent squash merge; do not alter installed Gimbal.

No fundamental contract question blocks planning now. Any issue phrase read as requiring excluded infrastructure implementation is interpreted under the user's explicit boundary: demonstrate runtime plumbing with the supplied fixture, and report backend behavior as fixture-supplied rather than implemented infrastructure.
