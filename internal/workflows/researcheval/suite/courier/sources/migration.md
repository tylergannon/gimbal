# Migrating Courier clients
The old v1 endpoint permits 20 messages per request. Version 1 and version 2 are available simultaneously during migration; a v1 limit does not describe v2 behavior.

In v2, accepted messages are retained for 9 days. In v1, accepted messages are retained for 3 days. Retention starts at acceptance, not the first delivery attempt. An acknowledged message is removed before the retention period ends. Messages are not restored by switching endpoint versions.

The v2 endpoint uses the same account identifier as v1. Clients should choose the version explicitly rather than relying on an undocumented default. Migration does not itself acknowledge a message. The documentation does not promise a date when v1 will be removed.
