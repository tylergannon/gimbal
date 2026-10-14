# Release critical path

decision: User goal is normal main-to-feed/Homebrew publication below ten minutes, aiming for five, preserving validation and Mac usability. Previous measured baseline is v0.13.12 at 764 seconds. Completion requires a published release with measured timings, all required checks, and observed Mac resource behavior.

decision: PR #538 merged the local-only mise cache setting. For the next slice, preflight selects scope and runs automation checks; a candidate tag is reserved before the full suite. The tagged release prepares source/frontend once, then platform packaging runs concurrently. Full Ubuntu qualification tests the versioned amd64 archive in the same job that compiled it, reusing its Go compiler cache. Publish depends on qualification and every platform. Generated inputs are tied to the exact Git revision and carry deleted outputs as well as newly generated files.

decision: Mac build work uses four Go/Zig/Xcode workers, with one release job at a time. The app bundles the versioned distributed CLI, reuses the prepared web build, prepares native libraries/project without Debug, then builds Release. Required CLI protocol, plugin, Go lint/vet/tests, frontend tests, formatting, Svelte checks, signing, notarization and publication checks remain.

friction: Pinned SKGO's native build unconditionally built frontend and Debug. A minimal upstream change adds prepare-only, reuse-web-build and jobs flags. Actual Gimbal preparation with these flags produced its Xcode project and native libraries without frontend compilation or a Debug product. Native package tests passed. CLI tests initially failed on ambient pnpm 12.11.2; VP's explicit environment matching the example's declared pnpm 12.9.1 passed the complete CLI test package.

decision: Native usability baseline was directly observed before the build: Finder screenshot capture and opening its View menu succeeded. Resource and interaction proof during the final published release are still required.

friction: Review caught that the runner uses system Python 3.9.6, which predates tarfile extraction filters. Input restore now validates member paths/types explicitly and passes on that exact interpreter. Linux's published archive contains bare gimbal (the Mac's signed/repacked archive uses ./gimbal); candidate qualification now extracts the full archive, accepting either spelling.

decision: Upstream SKGO PR #320 merged as 53179fc and its production/development qualification passed in run 38583689503. v0.33.0 is published and pinned in go.mod/go.sum with no local replacement. Its real CLI help exposes prepare-only, reuse-web-build and jobs. New archive web validation started the published v0.13.13 Mac CLI and received a successful Gimbal HTML response in 0.73s; every platform archive will run that check before publication in the new pipeline.
