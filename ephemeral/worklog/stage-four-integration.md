# Stage four integration

Separate display DTOs from editable opaque configuration before projecting
remote types; a custom JSON marshaler on configuration is not a derivation
contract for the frontend generator. Persisted transport details remain private.

Native instances load plugins only from their own catalog's plugins directory.
Disposable catalogs must not inherit the personal desktop's plugin selection.

Global Settings remotes need instance-level routing. Test the actual HTTP
entrypoint with no project Referer as well as a project page; component fixtures
cannot establish that routing. Repeated native actions also need a new request
even when their destination URL is unchanged.

Static-check cleanup explicitly discards best-effort temporary-file cleanup
errors, removes an unused tunnel mutex, and keeps operation-ID validation as an
ASCII allowlist. This changes neither ownership nor delivery policy.
