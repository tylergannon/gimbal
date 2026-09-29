# Authoritative user request

The requested deliverable is research and an algorithm/integration proposal for
the research-document workflow. The user asked:

> Do some research and find the information you need and produce enough
> information to make it so that when we run the research-document workflow,
> we're using Jev to check all of the claims in the semantic index, to ensure
> that it is true given the reference to the source data collected, and to mark
> the claims wherever they contradict one another. I don't care how many Jev
> requests are needed to accomplish that. Derive an algo and propose how to
> make it fit into our workflows. This is tricky bc our workflow language
> doesn't have this as a primitive. So either we change the workflow language
> or else we compile a separate gimbal entrypoint that can do the job as a
> RunCommand.

After the initial exhaustive-pair proposal, the user asked:

> Do we have to be O(n^2)? Is there a n*log(n) or better that we could use?
> if it's truly N^2 we'll do okay but just want to ask.

The assistant proposed source checking for every claim, structured grouping
where applicable, and exhaustive fallback where comparisons cannot safely be
excluded. The user then suggested:

> There should be a way to send some number of claims at a time, maybe ten or
> twenty or IDK where we start to lose fidelity, but you should maybe be able
> to do a batch and ask "are these claims coherent" and then if not we do a
> search for the contradictory pair, in that group????? IDK

The user requested consensus with Claude Fable on that direction. Batch size,
fidelity, and asymptotic improvement are questions to resolve, not established
facts. The original user requirement is to check and mark claims; it does not
require a separate API call or stored edge for every pair.

The user subsequently requested:

> probably want to write down the core premise and claims and have Sol do the
> proof. you need to do subagents or you'll run out of context soon

The candidate statements are in [core-claims.md](core-claims.md). Sol's
independent analysis is [mathematical-analysis.md](mathematical-analysis.md).

The user then challenged the batching premise directly:

> Actually batching might not work. how do you know whether two separate
> batches are the ones that contain the contradictory claims? You'd need a
> recombination practice that wouldn't degrade to N^2

The proposal must answer cross-batch/recombination coverage explicitly and
must not present reduced HTTP request counts as subquadratic semantic work.

The user accepted that limitation and suggested a bounded-input fast path:

> okay that's okay. We could do a check for whether if the claims all fit
> inside of 32k tokens, maybe you can do it in a single Jev request but that
> sounds hard eh

Evaluate that possibility without confusing context-window fit with judgment
fidelity or replacing the original per-claim source verification requirement.

The current proposal is [proposal.md](proposal.md); its source map is
[sources.md](sources.md). This work changes research notes, not the workflow
implementation. Repository instructions remain applicable.
# Implementation and evaluation follow-up

Tyler: "let's have `Sol` make that a factor in the research workflow(s).

Once complete, let's come up with an eval for research, and try to optimize it
a little bit based on the quality of the research, correctness of the semantic
index, and quality (queryability) of the semantic index.

This should be a gimbal workflow in and of itself --> come up with a measure,
and then try using maybe Opus 5.5 as the planner in a promise loop on this
evaluation, trying gemini models, deepseek, glm, and maybe luna / haiku, as the
research and indexing models etc.

Goal should be to find an economical solution that lends to success. We can
self-heal by looping on incorrect claims and such, but ideally we'd get
something that usually doesn't have to loop bc it tends to be correct, or that
only loops once etc etc.

but you ned to determine a WAY TO MEASURE it first. So, while Sol builds in that
correctness check and loop, you can work on how to evaluate it and probably
create a loop eval workflow."
