# Agent orchestration and observability references

Research date: 2026-09-17. Scope: three contrasting interfaces for Gimble's
run discovery, history, live participation, and workflow-map work. The
researcher initially had no browser rendering. The parent subsequently
inspected the Studio screenshot and Langfuse's embedded trace/graph images
in Chrome. None of this is hands-on testing of an authenticated product.

## 1. LangSmith Studio / LangGraph Studio — graph-first debugger

Sources: [Studio overview](https://langchain-ai.github.io/langgraph/cloud/how-tos/datasets_studio/),
[Studio usage](https://langchain-5e9cc07a.mintlify.app/langsmith/use-studio),
and a [LangChain community forum screenshot](https://canada1.discourse-cdn.com/flex007/uploads/langchain/original/2X/c/ce3ea4d43d1d4279b48e0a460d85714910a32e24.png)
(user-submitted, not marketing art).

**Visibly demonstrated / screenshot evidence.** The forum image is a public
LangSmith Studio capture attached to a report about a supervisor graph routing
into a billing subgraph and pausing for an order/invoice ID. Its surrounding
caption identifies an interrupt/resume screen; the author reports the same
step looping after Resume. This is a real screenshot reference, but not a
fresh run by this researcher. Parent visual inspection shows the graph at
upper left, an input form below it, and a separate right-hand execution
column containing a billing-agent interrupt, its question, and a response
editor. Zoom/fit controls and a small overview map are visible. The
[source discussion](https://forum.langchain.com/t/langsmith-studio-issue-when-resuming-from-an-interrupt-inside-a-subgraph-it-doesnt-properly-resume-instead-restarts/3160)
reports a bug; the image is useful for layout, not proof that resumption works.

**Documented interaction facts.** Studio has Graph mode (nodes traversed,
intermediate state, integrations) and a simpler Chat mode. The usage guide
documents a graph canvas, input form or raw JSON, streaming toggle, selectable
before/after breakpoints, Continue, Cancel, an assistant selector, a thread
selector, a detail slider, collapse/expand of turns/nodes/state keys,
Pretty/JSON views, Edit node state, Fork, and Re-run from here. Editing a
past checkpoint creates a fork rather than mutating history.

**Borrow / tradeoff.** Treat the map as an active debugger surface: selection
of a node should reveal its corresponding runtime detail. Do not import fork
or resume controls. For newly scoped stop-turn/cancel-run controls, the closest
documented precedent is Studio's explicit Cancel action for an ongoing run,
plus breakpoints that name both target node and before/after timing; these are
clearer than a generic emergency button. The detail slider and collapse/expand affordances are good
for large histories. Borrow the distinction between a graph definition and
thread/run state. Do not copy Studio's state-editing/time-travel controls into
Gimble unless they are separately authorized; Gimble currently needs review
and participation, not run forking.

## 2. Langfuse — trace tree plus session replay

Sources: [observability overview](https://langfuse.com/docs/observability/overview),
[trace best practices](https://langfuse.com/docs/observability/best-practices),
and [sessions](https://langfuse.com/docs/observability/features/sessions).
Public image URLs embedded by the official docs:

* [Trace tree screenshot](https://langfuse.com/_next/image?dpl=dpl_CQCrYXWnvGUss2Jb8SUUBZEbH1Pg&q=75&url=%2Fimages%2Fdocs%2Ffaq%2Fgood-trace-tree.png&w=3840)
* [Agent graph screenshot](https://langfuse.com/_next/image?dpl=dpl_CQCrYXWnvGUss2Jb8SUUBZEbH1Pg&q=75&url=%2Fimages%2Fdocs%2Ffaq%2Fgood-trace-agent-graph.png&w=3840)
* [Session view screenshot](https://langfuse.com/_next/image?dpl=dpl_CQCrYXWnvGUss2Jb8SUUBZEbH1Pg&q=75&url=%2Fimages%2Fdocs%2Fsession.png&w=3840)

**Visibly demonstrated / screenshot evidence.** Official image labels describe
a nested trace tree, an agent graph, and a session view. Parent inspection of
the first two embedded images confirmed the indented trace rows with timings
and the adjacent graph with Aggregated/Expanded choices, fan-out, counts,
and zoom/fit controls. The session screenshot remains an uninspected
supplemental link. These are published product screenshots.

**Documented interaction facts.** Langfuse models observations inside traces,
then groups traces into sessions. Its session view offers whole-interaction
replay, public sharing, bookmarks, and session-level scores. Best practices
recommend one generation per model invocation (interleaved with tool calls),
meaningful names, and filtering noisy observations. Inputs/outputs can render
as role-labeled conversation rather than raw JSON; metadata, model, token, and
cost fields remain inspectable.

**Borrow / tradeoff.** Use a stable run/session spine and a nested tree for
detail, while keeping the graph/map lightweight. The one-generation-per-call
rule is a useful warning for Gimble loops: repeated calls need distinct
runtime instances even when they share a call site/session. Session replay is
helpful for historical context, but a replay metaphor could imply temporal
playback; Gimble should present a static history with preserved context and
live-state badges instead.

## 3. Temporal Web UI — event timeline and live execution console

Sources: [Temporal Web UI v2.26 release notes](https://temporal.io/changelog/temporal-web-ui-v2-26-0),
[maintainer release](https://github.com/temporalio/ui-server/releases/tag/v2.26.0),
and a [Temporal 101 workflow-detail screenshot description](https://learn.temporal.io/assets/files/temporal-101-with-go-for-replay-2023-8f624a54b4f815fab55a7f121d65950f.pdf).
The release page's public image asset is:

* [Temporal workflow UI image](https://images.ctfassets.net/0uuz8ydxyd9p/2yF4xpQfk8WPLN7h3r8Inq/5fa5dac93d881c6142b8da2dacbb6478/320993356-fc3ddbe3-8d69-4ff7-90bd-fa2f80fcdf3c.png)

**Visibly demonstrated / screenshot evidence.** The official workshop PDF
describes a workflow detail page with a Completed badge, workflow/run IDs,
summary, input/results, tabs for History/Workers/Pending Activities/Stack
Trace/Queries, and an event-history table with timestamped rows and
expand/compact/JSON/download controls. This is an instructional screenshot,
not a fresh product session. The linked release asset is an official product
image, but its visual content was not available for hands-on inspection here.

**Documented interaction facts.** Temporal's v2.26 UI emphasizes quick
understanding, a live event-history feed, improved event correlation, better
large-history rendering/interactivity, and viewing child workflows without
navigating away. The release changelog mentions scroll-to-bottom behavior,
event-group styling, run-ID link navigation, and throttled refresh.

**Borrow / tradeoff.** A compact event/timeline view is a strong complement
to Gimble's graph: it preserves exact order, timestamps, and repeated events
without pretending the source graph is the execution history. Live-follow
should be an explicit mode with a clear jump-to-latest action; otherwise a
new event can steal the user's selection while reading older context. Child
workflow visibility is useful, but Gimble should preserve its distinction
between supervisor links, sessions, call sites, and execution instances.

## Three design directions for Gimble

1. **Graph-first debugger (Studio):** overview map + selected-node state
   inspector, with explicit live breakpoint/interview affordance. Best for
   understanding structure and active intervention; highest risk of implying
   unsupported mutation/time travel.
2. **Trace/session spine (Langfuse):** run list -> stable session/run header
   -> nested call tree -> detail drawer. Best for long histories, repeated
   calls, and preserving context while drilling down; avoid replay language
   and keep runtime instances separate from static call sites.
3. **Timeline console (Temporal):** run summary + event groups/timeline with
   optional map correlation and an explicit live-follow toggle. Best for
   concurrency, exact ordering, and hundreds of events; needs careful grouping
   so parallel branches do not look sequential.

Access note: the parent could view the Langfuse images on their official
page, although direct navigation to the deployment-specific image URL was
blocked. Use the source page if an embed fails. Temporal overlaps with the
execution research lane and is counted only once in the curated board.
No remote media was downloaded. Supplemental image links not explicitly
identified as inspected should not be described as visually verified.
