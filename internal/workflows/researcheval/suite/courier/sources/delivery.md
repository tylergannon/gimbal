# Delivery and retries
Courier v2 provides at-least-once delivery. A consumer can therefore receive duplicates; an acknowledgment lost in transit may cause the same logical message to be delivered again. The service does not promise exactly-once processing by a consumer.

Consumers should use the message identifier as an idempotency key when applying side effects. The identifier remains stable across delivery attempts for the same accepted message. This recommendation does not mean the server deduplicates all consumer operations.

On HTTP 429, wait for the Retry-After value before retrying. The capacity documentation's message count does not define a rate limit. A retried submission should preserve its own application request identifier. No retry schedule is given for an arbitrary consumer exception.
