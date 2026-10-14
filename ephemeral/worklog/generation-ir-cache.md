# Generation IR cache

correction: Each project owns its serializable generation results. Gimbal caches extracted runtime graphs, entry metadata, and the already-projected inspection page; ASTs/type identities and the inspector's pointer-keyed intermediate maps are not persistent products.

decision: Warm lookup still asks Go for selected files and compiler export identities. Cold parsing consumes captured root bytes, and publication compares the exports actually returned by the typed loader with the metadata used in the key. This prevents a dependency changing and changing back around the typecheck from poisoning a key for the original source. Dependency source is read only for the same-module import fact used by unread-helper diagnostics; compiler exports alone omit body-only imports.

friction: Rehashing every export archive made the tiny warm cached fixture slower than ordinary extraction. Go's own disk cache names immutable outputs by SHA-256; validated identities under effective GOCACHE avoid those duplicate reads. Other export stores, external package drivers, custom overlay/toolexec flags, and compiler-generated root syntax bypass this cache.

correction: Prior generator output is disposable, including syntactically damaged output. Package-only source overlays keep its declarations out of analysis, and existing header-owned malformed graph files can be repaired without requiring their old AST. Recoverable conflicting graph identities and authored files still refuse overwrite. A cache miss must retain these properties.

friction: Real Stock timing must compare fresh processes after Go's build cache is warm. An initial uncached invocation took 0.96s generation, while the matched later uncached invocation took 0.42s; claiming the initial value as the baseline would exaggerate benefit. Warm cached Stock took 0.33s generation / 0.34s process including live key metadata. This is Stock only: appgen still composes separate SKGO and Polytype work.

decision: Root source, comments, selected membership, module metadata, effective build configuration, logical locations, requests, source-import facts, actual dependency export identities, and the producer executable version the result. Atomic same-directory rename publishes only successful bound results; corrupt or inaccessible storage is a cache miss.

correction: Normal appgen now composes ordinary SKGO Run and Polytype codegen.Generate (TypeScript-only, explicit target and roots), allowing each project's cache to serve its own product. A shared upfront AST load bypassed that opportunity. Source/comment edits and current unsupported-type diagnostics remain checked through the public projection path.

friction: Whole-appgen integration initially remained slow even with Gimbal and Polytype hits: SKGO conservatively bypassed on a repository tooling symlink under .claude/skills. Do not infer integration speed from Stock-only timing or remove real repository layout to produce a benchmark. The SKGO owner is fixing source-selection handling. Likewise paired cache runs need the same environment; all-environment keys can invalidate solely because a subprocess inherits a different PWD or shell underscore value.

friction: Content-key filenames retain a new large Stock entry for every source edit and generator rebuild. Keep one replaceable slot per stable project/query, and validate the full source/config/producer key inside it; concurrent different versions can cause a safe miss, never reuse of the wrong product. The checksum additionally rejects semantic corruption that is still valid JSON.

decision: Known Go package errors from live metadata already match ordinary generation diagnostics; return them directly rather than repeat source loading. Unsupported acquisition or cache-storage failures still fall back. Tests compare undefined-name, syntax, and missing-import diagnostics with caching disabled and forbid repeating the checker.

decision: Retain live Go metadata on warm lookup. A cheaper mutable-dependency fingerprint alone cannot bind existing compiler exports to subsequently read source bytes across a compile-B/read-A race. Skipping that metadata would require capturing and compiling the mutable dependency closure itself; do not weaken source correctness to save that subprocess.

friction: The workflow JSON wire codec intentionally encodes nil slices as arrays. Reusing that representation for public Extract changed a nil Yields into an empty slice on a later hit. Cache internal generation products, whose emitters use the wire-equivalent shape; keep direct Extract on its original fresh Go-graph path. The warm regression first creates the emission cache and then checks the direct API's literal nil contract. Do not weaken the existing declaration test or build a second graph codec to hide this distinction.

friction: Local producer replacements are mutable inputs. Editing either producer worktree during Gimbal's cache population correctly rejects publication and repeats ordinary generation. Freeze source edits, not just benchmark commands, when measuring a cold-to-warm pair.

friction: Whole-appgen fresh-process recovery reached 0.64s, but the ordinary warmed go-generate directive took 1.22s including its wrapper and 0.27s external goimports. The updated SKGO producer emitted actual Gimbal Go files with an empty goimports -l result; remove that redundant directive only together with the supporting dependency pin.

decision: A large-consumer external GOFLAGS=-trimpath experiment reduced private-view cold generation from 20.40s to 10.27s, then 8.51s in a second distinct cache slot; all 392 observed files were identical. This is evidence for the SKGO owner, not a production flag change in Gimbal. It does not remove the need to bind analysis to captured source.

review: The recovered second review is static-only; it does not add execution or mutation evidence. It flags conservative PATH/HOME invalidation and missing isolated key-field mutations, while the body-only import test checks actual diagnostic changes through the production cache path. Distinguish a redundant key field surviving mutation from a demonstrated stale product; narrow environment identity only with concrete semantic and latency evidence.

correction: Ordinary generator tests also invoke default caches, including a consumer's released Polytype API. Isolate all three producer cache defaults in TestMain while preserving explicit child-fixture directories. Before/after metadata of the real user caches remained identical across both complete affected package suites.

failure: The same compiled generator invoked directly and by go generate missed solely because Go injects GOFILE/GOLINE/GOPACKAGE, explicit GOOS/GOARCH/GOROOT and a PATH prefix. An actual direct-to-go-generate-to-direct process test failed at the forbidden warm typecheck before the fix. Key effective Go settings and the resolved Go executable identity instead; retain checker/C configuration and live package/export validation. Both invocation transitions now hit and preserve the literal generated value.

correction: The user explicitly added a shared invocation-scoped Polytype/goload session alongside the three persistent project caches. Appgen must create one lazy session and each producer must consult its persistent result first. Gimbal's ordinary-build workflow roots, SKGO's evolving overlays and Polytype's tagged source are not interchangeable views. Preserve captured-byte and export-identity binding on a session hit; a reused graph cannot be required to fire the old parser callback again. No eager source preload or process-global session is authorized.
