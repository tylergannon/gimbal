# OpenCode harness research

correction: User wants implementation instructions focused on end state and definition of done, not a pedantic implementation checklist. Give workflow agents access to the whole corpus and semantic indexes, then leave routine design choices to them.

decision: User reversed async-only to simplify initial delivery: synchronous prompt POST plus shared SSE, with raw event capture and response correlation. Later Luna analysis may justify event-only reduction; do not discard POST authority yet. User explicitly requested Gimble workflows for implementation.

decision: Explicit model prefix selects OpenCode independently of model family: opencode/model defaults provider to opencode; opencode/provider/model selects the named provider. Preserve existing unprefixed routing and avoid a new model catalog.

decision: User selected shared OpenCode server autostart on first use, explicit stop interrupting active runs, and async-only prompting. Resolve completion/error ordering using that architecture; no synchronous prompt fallback.

decision: User chose the legacy API for the adapter. Keep one shared OpenCode server across projects/runs, controlled by `gimble opencode start|stop`, with runtime-configurable state defaulting to `~/.gimble/`. Verify source and settle remaining questions before implementation. Generate only required types; handwritten HTTP methods and SSE are preferred over further SDK codegen machinery.

correction: Legacy `/event` is directory/workspace-scoped; `/global/event` spans the server. The existing Gimble event callback already records correct run ownership, so a new adapter needs session routing, not another storage layer.

correction: Legacy idle is not a universal completion barrier. Fatal processor errors can emit idle before cleanup, structured output can update after assistant completion, recoverable compaction can emit session.error, and pre-run async failure can omit idle. Preserve these source-derived cases in live validation; do not copy CLI idle handling without its synchronous prompt completion signal.

correction: Legacy and newer APIs share SessionTable, but the inspected prompt input/message tables and runners differ. Different list results never proved separate session identity; shared identity likewise does not prove newer steering reaches an active legacy turn. Source traces found no such input bridge.

correction: GitHub release `target_commitish` is not authoritative for the release tag's actual commit. For OpenCode v1.18.27 it names b04697366f05419e9bd7a92f841813dd976161c9, while `git ls-remote` resolves the tag to 4b7e19e315cca414121ba1d61523fef74bb3ae8b. Resolve the tag before comparing package versions or claiming installed/source drift.

decision: Inspect the installed OpenCode server's `/doc` alongside the website. Version 1.18.27 declares both legacy endpoints and an experimental `/api` family with distinct prompt-delivery and durable-event contracts; a legacy-only comparison would miss decision-relevant capabilities.

correction: Gimble steering requires delivery into the active RunTurn. Consumption at the next model/tool boundary within that same turn can qualify; mid-token provider interruption is not required.

friction: Two research lanes wrote the shared final-report path before their assigned indexes were handed off, delaying synthesis. Steer researchers back to their owned topic directories and leave the complete report to the author role.

correction: The upstream Go SDK exists, but its inspected generated surface lacks fork, async prompt, schema format, and the newer API routes. Verify concrete SDK coverage rather than inferring availability from a JS/TS-only research corpus.

friction: Links from ephemeral/research/opencode to the local .gimble corpus require three parent levels. Check report reference targets after synthesis.

correction: User prefers the newer OpenCode API as the adapter foundation. Assess its functionality gaps first; cross-generation composition is a secondary option, not the default decision gate.

decision: The follow-up research must actually generate and compile candidate Go clients from installed `opencode generate` output and probe union behavior. Generation success alone is not evidence of a usable SDK.

correction: User previously wrote proprietary libopenapi-based OpenCode SDK generators; source is unavailable and must not be sought or reused. Evaluate current generators with bounded experiments; avoid prolonged workaround work when narrow custom generation is simpler. Future custom design can draw on user recollection via HITL.

correction: User wants the root model to coordinate and synthesize; delegate chores including generation, probes, compilation and validation to subagents. Root should review evidence and direct follow-ups rather than run experiments itself.

correction: User favors reusing generated Go types even if off-the-shelf client/server endpoint generation fails. Test models-only oapi-codegen/ogen before recommending custom struct generation; a hybrid generated-types/custom-SDK approach matches prior successful experience.

friction: Follow-up research-document run failed in index-curation on agy result arriving with unsettled tool step148; corpus preserved. Retry workflow using existing evidence rather than repeat generation experiments.

friction: Reusing the corpus did not avoid agy failure: retry run01M2XMQ94EEC1Y3XQ7CBAYP5GY.research-document ended during research with result arriving before tool step30 settled. Requested role-model replacement per gimble-runs instruction; no application repair in this research scope.

delivery: Two research-document attempts failed on the same agy unsettled-tool-step condition. Independent generator evidence review and author synthesis completed the report without making an OpenCode model turn; the durable conclusion is a proven oapi-codegen component-model layer (243 schemas, zero paths, compiling union tests) plus a custom HTTP/SSE boundary, with runtime execution still unproven.

delivery: Filed P1 bug #302 for the two observed non-finish AgY tools: run_command step/item 148 and view_file step/item 30. Root cause remains unconfirmed; saved runs retain normalized events but no raw native stream.

delivery: Reran the installed research-document workflow after the AgY fix using clean installed revision 9b192995b1a17b7fbdefd5297aa770b77e6f2818. Run 01M2Y1XE66XVD7C17N4H5G7MXX.research-document completed successfully with default Gemini 3.8 Flash medium researchers/indexer and Gemini 3.1 Pro high author/editor; editorial round 2 accepted no material findings. Reused completed evidence and generator trials; no new OpenCode inference or generator trials.

friction: The first author pass collapsed the requested compendium to 1,622 tokens and introduced unsupported contract/coverage estimates. Independent editorial rejection plus caller steering restored the full evidence-based report (10,027 tokens) and corrected source-versus-observation boundaries. Preserve the full brief during targeted corrections; verify indexes as well as final prose because unsupported operation-count/LOC estimates and step-versus-execution completion claims survived in intermediate indexes. Bounded rerun-cache corrections flag those estimates and prevent treating a step-ended event alone as turn completion.
