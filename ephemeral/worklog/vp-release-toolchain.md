# Vite+ toolchain correction

correction: User repeats standing rule: never invoke pnpm directly; use vp and let it manage pnpm versions. This rule was already recorded in memory but not retrieved/applied during the release continuation. Existing pipeline commands are not authorization to keep violating it.

decision: Replace CI Node/package-manager setup with official setup-vp; use vp install/run/exec/build/fmt throughout executable recipes and CI. Keep package-manager declarations/lockfiles as project input for Vite+ to manage. Add rule to AGENTS.md. Stop the in-flight release using superseded direct commands before dispatching the corrected source.
