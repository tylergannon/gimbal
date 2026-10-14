# Generation IR cache

correction: Each project owns its serializable generation results. Gimbal caches extracted runtime graphs, entry metadata, and the already-projected inspection page; ASTs/type identities and the inspector's pointer-keyed intermediate maps are not persistent products.

decision: Warm lookup still asks Go for selected files and compiler export identities. Cold parsing consumes captured root bytes, and publication compares the exports actually returned by the typed loader with the metadata used in the key. This prevents a dependency changing and changing back around the typecheck from poisoning a key for the original source. Dependency source is read only for the same-module import fact used by unread-helper diagnostics; compiler exports alone omit body-only imports.

friction: Rehashing every export archive made the tiny warm cached fixture slower than ordinary extraction. Go's own disk cache names immutable outputs by SHA-256; validated identities under effective GOCACHE avoid those duplicate reads. Other export stores, external package drivers, custom overlay/toolexec flags, and compiler-generated root syntax bypass this cache.

correction: Prior generator output is disposable, including syntactically damaged output. Package-only source overlays keep its declarations out of analysis, and existing header-owned malformed graph files can be repaired without requiring their old AST. Recoverable conflicting graph identities and authored files still refuse overwrite. A cache miss must retain these properties.

friction: Real Stock timing must compare fresh processes after Go's build cache is warm. An initial uncached invocation took 0.96s generation, while the matched later uncached invocation took 0.42s; claiming the initial value as the baseline would exaggerate benefit. Warm cached Stock took 0.33s generation / 0.34s process including live key metadata. This is Stock only: appgen still composes separate SKGO and Polytype work.

decision: Root source, comments, selected membership, module metadata, effective build configuration, logical locations, requests, source-import facts, actual dependency export identities, and the producer executable version the result. Atomic same-directory rename publishes only successful bound results; corrupt or inaccessible storage is a cache miss.

correction: Normal appgen now composes ordinary SKGO Run and Polytype codegen.Generate (TypeScript-only, explicit target and roots), allowing each project's cache to serve its own product. A shared upfront AST load bypassed that opportunity. Source/comment edits and current unsupported-type diagnostics remain checked through the public projection path.

friction: Whole-appgen integration initially remained slow even with Gimbal and Polytype hits: SKGO conservatively bypassed on a repository tooling symlink under .claude/skills. Do not infer integration speed from Stock-only timing or remove real repository layout to produce a benchmark. The SKGO owner is fixing source-selection handling. Likewise paired cache runs need the same environment; all-environment keys can invalidate solely because a subprocess inherits a different PWD or shell underscore value.
