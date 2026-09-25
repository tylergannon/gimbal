# Jev readiness for Gimbal

2026-09-23. This is a decision note, not a product integration or a claim that Jev improves Gimbal outcomes.

## SDK choice

Use [`github.com/kazz187/jev-sdk-go` v0.2.0](https://github.com/kazz187/jev-sdk-go/tree/v0.2.0) for a first Go experiment. It is an **unofficial** TypeSafe client. TypeSafe currently publishes [JavaScript and Python SDKs](https://docs.typesafe.ai/sdk), with [HTTP](https://docs.typesafe.ai/api) as the language-neutral contract. Gimbal already requires Go 1.27.1, which meets this client's Go 1.27 requirement.

| Client | Fit for Gimbal | Main limitation |
| --- | --- | --- |
| [`kazz187/jev-sdk-go`](https://github.com/kazz187/jev-sdk-go) | A `Choice` over a caller-defined string type returns that same type; heterogeneous batch handles preserve each answer type. It has explicit retries, model pinning, request usage, a fake provider, and no external modules. | Very new community package. Go types constrain answer shape, not semantic correctness or exhaustive `switch` cases. |
| [`Stumble/jev-go`](https://github.com/Stumble/jev-go) | Mature-looking transport tests, a CLI, security-minded endpoint handling, Go 1.22 compatibility, and its own integration skill. | Answers are a tagged union reached through string-keyed maps, so call sites must check the discriminator and fields. |
| [`Shubham510/typesafe-go`](https://github.com/Shubham510/typesafe-go) | Straightforward standard-library transport, typed answer accessors, retries. | Questions and answers still meet through string names; it does not carry the caller's Choice enum through the result type. |
| [`kataras/jev`](https://github.com/kataras/jev) | Has account-level rate pacing and a generic `SystemOneAs[T]` response decoder. | The caller must write the response struct separately from the question; it adds `golang.org/x/time`. |
| [`guchengod/typesafe-sdk-go`](https://github.com/guchengod/typesafe-sdk-go) | Broad transport surface and CLI, with a generic response decoder. | Question IDs and answer extraction remain string keyed. |
| [`unimtx/typesafe-sdk-go`](https://github.com/unimtx/typesafe-sdk-go) | Small standard-library client with validation and conventional accessors. | Question IDs and answer extraction remain string keyed. |
| [`havlan/jev-go`](https://github.com/havlan/jev-go) | Minimal standard-library client. | Smallest surface and fewer integration aids; answers are string keyed. |

All seven cloned heads passed `go test ./...` locally on 2026-09-23. A direct TypeSafe smoke call through `kazz187/jev-sdk-go` v0.2.0 returned model `jev-1.13.0`, probability `0.990` for a simple failed-test question, and 309 input tokens. That proves basic authentication, request serialization, and response decoding for one case. It does not test supervision quality. The key was supplied to that process through hidden terminal input; it was not put in a file or the repository.

Do not add an unused dependency to Gimbal yet. The first experiment can import the pinned module in its own small Go package when there is an actual consumer. Keep `TYPESAFE_API_KEY` in the server-side process environment or a secret store. Pin both the SDK version and `jev-1.13.0` while measuring judgments, and record the concrete model in each response. If the community SDK becomes a maintenance risk, the [small HTTP contract](https://docs.typesafe.ai/api) is the fallback; no special Gimbal abstraction is needed merely to make a request.

For question design, use TypeSafe's [official agent skill](https://github.com/typesafe-ai/skills/blob/main/skills/typesafe-ai/SKILL.md) and live docs first. The [Stumble Go adapter skill](https://github.com/Stumble/jev-go/blob/main/skills/jev-go/SKILL.md) is useful only if choosing that SDK; do not conflate its API with the chosen client.

## Best first use: frequent supervision screening

The [draft message and cooldown contract](supervision-message-contract.md) specifies the proposed per-rule Jev request, the generative-supervisor handoff, and the two cooldowns.

Today [`WithSupervisor`](../../../supervise.go) starts a generative supervisor look on a timer (three minutes by default). Each look reads a bounded recent transcript; an objection is sent as a free-text `Steer`. Jev can judge a smaller, named snapshot more often and decide **whether to call that supervisor**. It cannot write the situation-specific objection. The existing runtime owns the transcript buffer and steer lifecycle, so this screening belongs alongside that machinery if an experiment justifies integration.

Every provider-exposed completed reasoning message should trigger one separate Jev request per attached supervisor rule, using the bounded state and Choice question in the [message contract](supervision-message-contract.md). Candidate rules include:

- Does the worker's latest plan conflict with an explicit task constraint?
- Does its completion claim conflict with the observed tool or test result?
- Is it repeating the same approach without new evidence, even if the words or commands differ?
- Does a new tool result suggest the worker needs a diagnosis beyond a prewritten correction?

Code should use exact facts it actually observes and leave semantic judgments to Jev. Jev's output is an advisory screening distribution, not a calibrated probability that the agent is out of bounds. The proposed cooldowns gate live generative reviews and automatic steers, while Jev still checks every completed reasoning event. Cap concurrency and spend, discard results after the worker turn ends, and let the existing supervisor cadence handle API failures. Continue to supervise normally while Jev is in shadow mode.

The first experiment is offline replay over saved Gimbal turns with independently labeled moments requiring intervention, including healthy temporary failures and productive exploration. Compare exact Go signals, current cheap generative supervisor, and Jev questions at the same decision points. Measure false escalations per healthy run, recall of real intervention points, lead time, latency, and actual cost. Tune thresholds on one slice and assess on held-out runs; do not assume TypeSafe's confidence is calibrated for Gimbal. Only then try live shadow judgments, then Jev-triggered generative review. Automatic steering would require separate evidence that a particular intervention is safe and helpful.

For early external examples, see [prior art](prior-art.md).

## Other promising uses

1. **Evidence selection for research and semantic indexes.** Retrieve candidate excerpts deterministically, then use Jev to rank relevance or flag contradictions before a generative researcher reads them. Keep original text, source path, and citation intact. TypeSafe's [reranking](https://docs.typesafe.ai/cookbooks/rerank_typesafe) and [RAG passage classification](https://docs.typesafe.ai/cookbooks/classifying_rag_passages) cookbooks demonstrate the shape; their benchmark gains do not transfer automatically to Gimbal.
2. **Claim-versus-evidence checks during validation.** For a claimed fix, first verify files, commands, and exit codes in Go. Jev can then judge whether cited source passages or observed output actually support the claim, escalating uncertain cases to the validator. This follows the [citation-check](https://docs.typesafe.ai/cookbooks/citation_check) pattern. It cannot replace the validator's acceptance judgment.
3. **Routing bounded work.** A closed Choice can suggest a workflow, skill, or model tier for a task. Keep the full task for the chosen agent and shadow-test routing decisions against quality and cost before changing dispatch. TypeSafe documents [intent routing](https://docs.typesafe.ai/patterns/intent-routing) and [skill suggestion](https://docs.typesafe.ai/cookbooks/skill_suggestion).

## Limits that matter here

Jev returns `Noul`, `Choice`, or `Score` judgments over text or JSON state. It does not generate prose or inspect images/audio/video directly. [Model documentation](https://docs.typesafe.ai/models) currently lists `jev-1.13.0`, $0.042 per million input tokens, free output, 64k total request tokens and 32k for state plus the longest question; published limits are dynamic. Those prices make a bounded screening loop plausible, but do not establish end-to-end savings because snapshot size, frequency, retries, and false escalations matter.

Cost scales with completed reasoning events × attached rules × measured input tokens per request. The downstream generative reviews may dominate total cost, so count those as well as Jev requests.

TypeSafe's own [jaggedness notes](https://docs.typesafe.ai/model-jaggedness/jev-1.13) warn about counting, arithmetic, distracting long state, indirection, and adversarial text. Treat the worker transcript as untrusted data, and never let a confident answer substitute for observed evidence. TypeSafe says customer requests are not used for training, while [zero data retention](https://docs.typesafe.ai/legal) is an enterprise offering; decide what proprietary source or transcript data may leave Gimbal before a live rollout.
