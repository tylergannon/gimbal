# Go cache ownership

friction: Docker creates missing parent directories for nested bind mounts as root. Mounting shared modules at go/pkg/mod left go/pkg root-owned, preventing the runner from creating private checksum-database state. The archived-plugin check caught it. Precreate and own go/pkg in the runner image; apply ownership correction to the running containers. Preserve module verification and the candidate check.
