# Shared generator inputs

SKGO v0.30.0 and Polytype v1.6.0 expose caller-owned, type-checked package
handoffs. Gimbal's appgen composes stock generation and the web/type backends
through their library APIs; it must take its tagged domain snapshot only after
stock has published its route inputs.

Keep each snapshot local to one invocation and one source view. SKGO's normal
route discovery and intermediate overlays differ from Polytype's jsonschema
view and legitimately load separately. Passing an incompatible snapshot does
not force reuse; SKGO falls back. Confirm actual hits with OnPackageLoad when
changing this composition rather than inferring reuse from a supplied snapshot.

The released SKGO snapshot constructor needs dependency file metadata, and its
validated loading path uses NeedDeps. Polytype FromPackages itself needs syntax
only for packages reached by lowering. A cheaper dependency-metadata handoff
would need an upstream API contract; do not fabricate provenance by stitching
separate type universes together or retaining a global cache.

Observation's public frontend roots are selected in appgen. There are no
observation JSON owner codecs or schema accessors to regenerate. Workflow's
sealed Graph does have generated Go JSON codecs, so retain its Polytype CLI:
the public grammar constructor plus TypeScript renderer cannot replace that
output. SKGO prunes separately owned frontend type files, so publish those
afterward and run the workflow generator after the app generator.

Keep the go:generate declaration's generate token and hook/locals/Swift
selections readable by SKGO's native and dev entry points. The declaration test
checks this library-composition path as well as the native remote selection.
