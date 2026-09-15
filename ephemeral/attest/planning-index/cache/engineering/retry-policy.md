# Retry delay policy

retry_delay(attempt) returns seconds. Attempts are zero-based. Return 7 * min(max(attempt, 0), 4). Thus attempt zero has no delay, attempts 1/2/3 wait 7/14/21 seconds, and attempts 4+ wait 28 seconds. Negative attempts have no delay. This helper only computes a number; it must not sleep, schedule jobs, or perform IO.
