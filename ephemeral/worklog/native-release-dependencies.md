# Native release dependency recovery

correction: The first PR #540 release reached the native preparation step but failed because the clean Mac checkout had no `web/node_modules/@sveltejs/kit/src/utils/routing.js`. Native Swift-binding generation still reads installed Kit route metadata even when the existing frontend build is reused. Local proof had retained node_modules and missed this clean-runner condition.

decision: Restore pinned pnpm/Node setup and frozen dependency installation on the Mac. Keep the pnpm store local, with no remote cache transport, and keep frontend/Debug rebuilds removed. This is the smallest recovery; no second upstream release or new native-generator mode is needed.

validation: actionlint v1.7.7 passes. Starting with node_modules removed, pinned pnpm 12.10.1 rehydrated all 242 dependencies from the local store in 2.8s with zero downloads; real SKGO v0.33.0 native preparation then completed successfully, including Swift generation, native libraries and Xcode project, without frontend or Debug compilation. Final proof remains an actual successful signed release and measured end-to-end timing.
