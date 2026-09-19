# Scope-owned services

decision: The prior-art workflow settled issue 276 without a separate planning run: Service is a direct asynchronous declaration, successful start is not readiness, and any exit before intentional shutdown including zero fails and cancels the owning scope.

decision: Service launches `zsh -c` in a dedicated process group. Normal close and cancellation mark shutdown before signaling, send SIGTERM to the group, wait five seconds, escalate to SIGKILL, wait five more seconds, reap the direct child, and require the group to disappear. The portability promise ends when descendants escape the group.

decision: Reuse CommandStarted and CommandEnded plus run-owned command output artifacts so a live and finished service appears on the existing command observation surface. Static workflow extraction represents Service as workflow.Command.

design_bug: Iterate previously discarded its child scope result because its iterator callback cannot return an error. A required service that failed inside an item would therefore disappear and iteration could continue. Propagate that child service failure into the enclosing scope and stop iteration.

design_bug: Binding shutdown to the context argument passed to Service let a caller cancel a derived call context and silently stop the required process while its owning scope remained live. Reject an already-cancelled call, but bind ongoing ownership to the scope's canonical context.

design_bug: An unexpected exit classified before shutdown could block while recording its CommandEnded event, letting scope shutdown begin before the error reached the scope. Install the classified failure and cancel the scope before persistence; shutdown cannot relabel that decision.

friction: The restricted sandbox blocks local sockets and the Go build cache, so the complete build, vet, test, and race proof must run with the repository's already-approved external test permissions.

friction: The first full test run hit one unrelated watch-stream `unexpected EOF`; the exact test passed ten immediate repetitions and the clean full repository rerun passed, so there is no evidence of a branch regression.

correction: Run the repository's Lefthook pre-commit gate rather than treating direct gofmt as the formatting authority. The hook applied the pinned goimports and modernize tools, then passed staged-content policy, both linters, Go tests, and all web checks.

proof: Targeted lifecycle tests cover an enclosing service with one PID across iterations, item-local shutdown, normal and cancelled SIGTERM traps, synchronous start failure, unexpected zero-exit scope failure, command observation, and SIGKILL cleanup of a TERM-resistant descendant. Five race-detector repetitions passed.

proof: A temporary uncommitted workflow used Codex gpt-5.6-luna without a workflow model override. Its service passed a real `kill -0` usability command, retained PID 33073 across two item scopes, the agent returned SERVICE_OK, normal close and cancellation each wrote TERM, a missing workdir failed synchronously, and an unexpected exit zero failed the run.

validation: Gimble review run `01M2W467KDS91C9QJCNX927WRE.review` used the workflow's configured `code-review` default, Codex gpt-5.6-luna at high effort, with no model override flag. After live steering stopped further investigation, it returned `{"findings":[]}` and the run ended successfully. The turn was not operationally bounded enough: it ran 5m22s and repeatedly searched the downloaded corpus before the steer.
