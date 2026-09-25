# Comparison evidence clip

The full downloaded primary sources remain in `../sources/`. This clip keeps
the lines most useful to issue #284's narrow static-shape question.

- Pulumi: `pulumi-parent.md:7-18` defines an explicit parent, an implicit root
  stack, and multiple nesting levels; `:106-118` shows the hierarchy as an
  indented tree. It is the clearest ownership precedent, but its lifecycle
  inheritance is not a Gimbal runtime requirement.
- Kubernetes: `kubernetes-owner-references.md:9-31` separates owner/dependent
  membership from labels and stores the owner name plus UID in metadata. It is
  a precise identity/reference precedent, but its garbage-collection rules in
  `:64-84` are deliberately outside the static Gimbal graph.
- GitHub Actions: `github-actions-docker-services.md:16-24` says services are
  configured per job and destroyed with the job; `:70-96` puts named services
  beside the job's steps and uses the configured label as the access name.
  This is the strongest non-step/owning-scope precedent.
- Docker Compose: `docker-compose-services.md:8-18` makes `services` a named
  map of computing resources; `:30-80` shows named entries beside the
  application's command-bearing configuration. `:412-477` shows that
  `depends_on` is a separate optional relation, so a service list need not
  become a dependency graph.

The transfer is intentionally only shape: parent/member containment, stable
names, and a visually separate resource collection. Do not import readiness,
health, restart, garbage collection, process, or dependency semantics.
