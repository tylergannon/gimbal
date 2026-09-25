Okay so yeah I think this is amounting to an argument for how to re-engineer Gimbal, away from a kind of dummy copy of strongdm's atgimbal, into a full-on reaction to the experience of using that pattern and rubbing up against just how fucking hard it is to design a good workflow using it and -- worse yet -- the lack of intuition that coding agents seem to have on how to make a GOOD ONE.





I think success here will therefore come from giving humans and agents alike the ability to easily understand the *context engineering* ramifications of any given workflow shape.





Does that give you any intuition on where we're going?  I feel like in some way we're going to want not only to be able to have fluency developing workflows, but we're also going to want to be able to collect specific types of telemetry, like it would be really cool if we could analyze a run that failed or seemed to take too long or too many steps, by looking at the initial prompt, the quality of the semantic index and the data that it indexes, and the actual prompt, as well as the number and nature of steering messages.



We don't have to build that in to the first version, it's just a concept of where I'd like to see us heading.  All of this should go not just into like "workflows as programs" but more generally, into our "direction" docs.



Okay.



One more significant thing to point out is that there IS a very mother fucking good reason for the DAG-style workflow definition:



> When you look at a good DAG workflow design, you know what the workflow is going to do and how it's going to progress through its stages.



I think that turns out to carry a fallacy with it, that it's therefore going to be easy to develop good successful workflows.  But that promise of legibility is something that we should really strive to preserve.



I think at their best, a good workflow design should and might basically read like pseudocode.  And I think it's really important for us to bear that in mind as we develop the functions and guidelines/skills for how to develop them.  Indirection, composition, DRYness, function and type names, etc, should all be evaluated for whether they bring us closer to the ability to write a workflow where you can look at a page (or so) of code and get a GREAT sense of the workflow shape.



Whether or not we can render it into a graph using automatic tools I'm not sure.  WDYT?



please incorporate and respond to these thoughts.
