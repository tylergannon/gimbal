# Issue 130 clarification

correction: Tyler confirmed durable reduced snapshots with exact event positions are the purpose of the store, including after server restart. The old no-checkpoint and every-connection-reset decisions must not override this requirement.
decision: Implement the approved direction in ephemeral/plans/issue-130-durable-snapshots.md; Sol owns execution and evidence.
