# Independent compiler extraction validation

correction: A passing defect-reproduction overlay is not a passing requirement: new recordPlan and WriteContext paths returned diagnostic event-write failures, violating observation/execution separation. Retain recording diagnostics while allowing successful semantic operations to continue.
correction: Temporal RequestCancel can return nil for a retained terminated run. Report accepted delivery separately from controller completion and resource cleanup.
friction: TestLiveLargeTypedResult needs SPECIMEN_LIVE_TRANSPORT=1 as well as a Temporal address; the address alone silently skips it. Always inspect skip output before claiming transport proof.

correction: The observation-authority regression was repaired by the implementation worker and independently rechecked: Generate continues after failed diagnostic writes, committed context is preserved, and the final recording diagnostic remains available.
