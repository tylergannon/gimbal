# Execute the release recovery and Python replacement plan

Tyler authorized execution of the reviewed plan on October 14, 2026. The authority
is diagnosis.md's remediation plan and repository AGENTS.md. Deliver observer
recovery first and verify it in the real workflow, then complete the remaining
Python replacement. Phase one intentionally retains the hook and other Python
entry points until their replacement is delivered; the overall task is not
complete at that checkpoint. Documentation edits are limited to maintained
invocation/runtime instructions required by the migration.

Implementation and validation observations are reported in chat and PRs. Raw API
responses, downloads, command outputs and probe files stay outside Git. Reviews
and actionable worklog notes remain here. No new public Gimbal API is requested.

The observer-first checkpoint merged in PR #552. Its trusted-main observer and
independent hourly job were subsequently observed to pass, with uploaded reports;
recovery was verified before the remaining ports began. The second delivery
replaces all remaining maintained Python helpers and updates invocation/runtime
instructions. The release jobs explicitly poison Python lookup before maintained
commands, so native runner execution verifies the new prerequisite boundary.
