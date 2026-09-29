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

The proposal is [proposal.md](proposal.md); its source map is
[sources.md](sources.md). The follow-up implementation adds the compiled
claim audit and research evaluation workflow. Repository instructions remain applicable.
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

# Provider and role scope update

Tyler: "okay maybe the Diffusion Router will turn out to be too slow for our purposes right now. That's a 'deferred execution' router for most of the models, I think. We need to get moving so let's just work with the big three providers gemini, openai, claude for now and not get hung up on GLM / DeepSeek"

Tyler: "Yeah, let's add Tara from ChatGPT as well as Claude Sonnet. Also, I think there are a couple of different roles in this that we should be mixing and matching in this eval process because, right? Like there's the guy who goes and does the research and downloads information from the internet, and then there is also the guy who actually builds and balances the semantic index. And I'm not sure where the weights are on this, but I imagine that one of those jobs is harder than the other in terms of the amount of work that needs to be done, and one of them is harder than the other in terms of the cognitive load to make the judgments needed in order to balance the index, or something like that."

'Tara' is interpreted as the installed OpenAI Terra family. Research and index model bindings are independently variable; source acquisition and index judgment workload should not be conflated.

# Release priority

Tyler: "I kind of need to get a new release of this workflow sometime soon so that I can use it. ... you are forbidden from perfecting this benchmark, and your instruction is to, for the time being, just use some kind of easy-to-work comparison so that we land on a set of model assignments that are going to get us a reasonable, viable outcome when we run this workflow. We can think more about perfection in our benchmarking another time."

Finish a small fixed-source comparison and release the usable research workflow. Broader web-discovery benchmarking, exhaustive combinations, and statistical repetition are deferred.
