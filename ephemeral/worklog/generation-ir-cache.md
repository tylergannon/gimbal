# Generation IR cache

correction: Each project owns its serializable generation results. Gimbal caches extracted runtime graphs, entry metadata, and the already-projected inspection page; ASTs/type identities and the inspector's pointer-keyed intermediate maps are not persistent products.

decision: Warm lookup still asks Go for selected files and compiler export identities. Cold parsing consumes captured root bytes, and publication compares the exports actually returned by the typed loader with the metadata used in the key. This prevents a dependency changing and changing back around the typecheck from poisoning a key for the original source. Dependency source is read only for the same-module import fact used by unread-helper diagnostics; compiler exports alone omit body-only imports.

friction: Rehashing every export archive made the tiny warm cached fixture slower than ordinary extraction. Go's own disk cache names immutable outputs by SHA-256; validated identities under effective GOCACHE avoid those duplicate reads. Other export stores, external package drivers, custom overlay/toolexec flags, and compiler-generated root syntax bypass this cache.

correction: Prior generator output is disposable, including syntactically damaged output. Package-only source overlays keep its declarations out of analysis, and existing header-owned malformed graph files can be repaired without requiring their old AST. Recoverable conflicting graph identities and authored files still refuse overwrite. A cache miss must retain these properties.

friction: Real Stock timing must compare fresh processes after Go's build cache is warm. An initial uncached invocation took 0.96s generation, while the matched later uncached invocation took 0.42s; claiming the initial value as the baseline would exaggerate benefit. Warm cached Stock took 0.33s generation / 0.34s process including live key metadata. This is Stock only: appgen still composes separate SKGO and Polytype work.

decision: Root source, comments, selected membership, module metadata, effective build configuration, logical locations, requests, source-import facts, actual dependency export identities, and the producer executable version the result. Atomic same-directory rename publishes only successful bound results; corrupt or inaccessible storage is a cache miss.
