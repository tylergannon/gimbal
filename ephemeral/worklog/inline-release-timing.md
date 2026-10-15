# Release and commit-hook timing

correction: Tyler rejected the standalone Release performance observer. Release timing belongs inside the release workflow and should complain there. Commit-hook timing must warn the committing agent directly without failing; fail with instructions only if non-failing visible warnings cannot be made to work.

decision: Remove the scheduled observer and its historical scanner/issue creation. Each release workflow reports its own elapsed time with Actions warnings and a job summary. Lefthook follows command output and explicitly enables execution output; hook overruns write actionable warnings to stderr while retaining success status. Correctness failures still reject commits.
