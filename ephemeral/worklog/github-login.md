# GitHub login delivery

decision: Implement the reviewed optional-auth design from the linked chat; current authorization additionally requires a real GitHub App and working OAuth restricted to Tyler's GitHub account.
friction: Thread read bounds are turnLimit <= 10 and maxOutputCharsPerItem <= 20000. Filter returned messages before printing rather than dumping command transcripts.
decision: Authentication evidence is observed through the running application and reported in chat/PR; no run output or proof programs are committed.
correction: Tyler explicitly wants http://localhost:PORT and questioned HTTPS on localhost. Permit explicitly configured HTTP loopback origins with host-only HttpOnly SameSite cookies; HTTPS and __Host cookies remain required off loopback. This overrides the reviewed design's local HTTPS requirement.

correction: Tyler scoped review blockers to broken or incomplete behavior, bypassed critical interfaces/component seams, and obvious insecurity; speculative correctness refinements do not hold up delivery.
friction: The native agent reviewer was blocked by a Claude rate limit; an independent session reviewed the complete implementation instead.
