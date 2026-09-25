# Selected text:

## Selection 1: /Users/tyler/.codex/worktrees/0f95/gimbal/docs/workflows-as-programs.md (lines 54-62)
Working context will eventually grow too large to send in full with every
request. At that point, information should spill onto disk, where the agent
can find and inspect it as needed.

This is a continuation of the semantic-index approach already explored in
Gimbal. The program's active context should be treated as a changing,
evolving addition to the first or root node of the semantic index available
to that agent. As execution advances, this entry point connects the agent's
current work with the supporting information it can retrieve.

## My request:
I want to push back / edit.  I need you to reframe the context / semantic index.



It's not to say "working context will eventually grow\...."



You're right and that's true, but I'm trying to shift the framing in what I think is a really important way.



What I'm trying to point out is that there's only one context for a given agent.  Not two.  I'm trying to break down the dichotomy between the two types of in-scope information.



This kind of thinking is correct but it misses out on something that I'm hoping you can help me to capture properly:

> At that point, information should spill onto disk, where the agent
>
> can find and inspect it as needed.



The point I want to try to draw out is, maybe this is the place where the "program" metaphor breaks down.  If we cling too hard to the "program" metaphor then it pulls focus away from the thing we're really trying to optimize, which is to get each agent to achieve its goal, (a) more accurately, (b) faster, (c) for less money (tokens).



So at the most important level we're just doing context engineering.



Quite possible that's our most important point to make.  This is NOT an exercise in trying to fully realize the metaphor.  It's all about trying to make sure that the agent has the information that it needs, when it needs it.



And IMO right now the most important thing that I understand about context engineering is that you want to maximize the clarity of the objective while giving the agent really great pathfinding tools for discovering the information that it needs rapidly before filling up its context.



At this point, this might be a distinction without a difference?  Because I'm not sure it changes our tactics.  But I want it to drive our thinking.



In some way, if the "semantic index" and the information about the goal, the rules, the means, the requirements, etc etc, were all one unified thing, I have a feeling that this would be a way to try to optimize the agent's path to success.



At the same time, we EXPRESSLY RESTRAIN ourselves from trying to get one agent to remember everything about what it's supposed to do and how it's supposed to get done.  We divide the knowledge up across space and time, by having a workflow in which a checker agent follows a builder agent, and in which supervisor agents are observing from a different level, to look for specific problems either in space -- a supervisor agent notices the builder getting into overengineering and helps to steer/guide the builder -- or in time -- a validation agent receives the work and gives it a failing grade.



I'm failing to come up with a powerful conclusion to these remarks, that ties them together and gives them their thrust.  Can you help?   We just did context compaction so you'll have to re-read at a minimum the [workflows-as-programs.md](docs/workflows-as-programs.md) and possibly refresh yourself on the topic in other ways. &#x20;



I guess what I'm wondering is if we can somehow make the on-disk information a first-class citizen alongside all of our orchestration, and design the information we pass in the first message of a prompt, to consider not just the runtime state of the loop we're in, but the abstract goal of SUCCESS that we always have, when calling an agent.
