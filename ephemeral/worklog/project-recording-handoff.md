# Project switching and recording ownership

decision: Implement the approved Gimbal View stage-two plan from its 13 pinned Designer cards; Runs tabs remain stage three.
decision: Reuse the controller's drain/flush/close operation, awaiting its outcome. A second close is not the Designer card's intended behavior.
decision: Persist monotonic active duration; old metadata without duration stays unknown. Carry persisted requirements mode, including marking during capture.

decision: A close/display-confirmation failure retains a retryable save outcome and session identity; abort is reserved for source/delivery failure. The next project request cannot treat failed saving as idle.
decision: Destination navigation receipt establishes the selected project independently of old transcript observation. New capture waits separately for display; a failure belongs to the selected destination.
friction: Native generation and the appgen directive both select Swift remotes; update both lists and use TestNativeSelection to prevent post-build generation from removing a required query binding.
decision: Durable close must reject interrupted metadata after server restart; a generic closed acknowledgement is insufficient to prove successful saving.

decision: Terminal close/abort retry reconciles both durable files before acknowledgement. A full-record write alone cannot prove saved history/duration; metadata may have failed independently.
correction: Native dark acceptance found existing .dark tokens were never selected. Root layout follows the system color scheme, activating those tokens without a separate theme setting.
correction: Local native validation repeatedly requested microphone permissions. `just app` generated CODE_SIGNING_ALLOWED=NO and left linker ad-hoc signing with a cdhash-only designated requirement, while installed Gimbal uses Developer ID. Stable bundle name alone does not preserve privacy grants. `just app-validate` now signs with the existing Developer ID and release entitlements; ordinary `just app` remains available without a certificate. Native validation instructions require the signed target; never reset TCC or rename the app to work around validation. Verify signature identity and actual permission reuse separately from a preparation-timeout hypothesis.
