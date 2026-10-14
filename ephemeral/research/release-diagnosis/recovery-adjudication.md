# Recovery review adjudication

Round 01 was an implementation review before merge, not a declaration of
post-merge recovery. The authoritative sequence is merge, dispatch the trusted
main performance workflow, then verify both jobs and artifacts. That evidence is
outstanding and must be obtained before starting the remaining migration.

The review identified four advisory issues that recovery would newly create.
The 725-second v0.13.15 delivery missed the existing 600-second maximum. Tags
v0.13.17/18/19 each have only failed/cancelled tagged runs and no published GitHub
release. They are genuine historical failed deliveries, not successful delivery
timestamps missing from the scan. Existing policy deliberately reports sources
that remain unpublished, counting elapsed queue/retry time until publication,
and deduplicates advisory incidents by source. The original Python observer had
that same policy. Do not move its adoption cutoff to hide the incident backlog.
No current publication failure or active job is inferred from these records.

Clarified issue wording from 'still pending at' to 'not published by' the
observation timestamp: a historical failed delivery is not asserted to be an
active release. Preserve original timestamps and expose observations rather
than inventing a terminal delivery time. Historical timing incidents remain
advisory and do not gate release publication.
