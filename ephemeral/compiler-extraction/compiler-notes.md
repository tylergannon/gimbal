# Compiler extraction notes

decision: Stage 2 keeps admission and graph generation private. Split-response type admission lives in internal/compiler; Temporal formatting/replay effect checks remain in the emitter.

decision: Graph regeneration recognizes this generator's ownership header and existing Graph entry/name metadata. Stale generated code may be replaced, but handwritten output, another workflow identity, and another registration in the authored package are rejected. The previous test that replaced the entire output with unmarked handwritten code now retains ownership metadata while introducing a stale unresolved symbol.

doc_bug: Planning's missing production import was not the only graph gap: Results had no generated authored graph. Temporal generation now invokes GenerateGraph for every authored entry and always imports that package, even when no authored types are referenced. This establishes build-time registration; it is not live browser evidence.

decision: GenerateGraph omits stock CLI/Form parameter-field and flag restrictions. It preserves extractor diagnostics in the graph rather than treating visualization coverage as executable lowering permission.
