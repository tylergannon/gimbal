# Conventional releases

decision: Tyler requested Conventional Commit version selection and rejection of breaking-change declarations in the local commit hook. Gimbal is pre-release with no users; major version increments are forbidden. Classify pre-release API changes as features or fixes.
correction: Automatic publication introduced on 2026-10-13 hardcoded patch bumps; automation must preserve meaningful version selection.
decision: feat selects minor, fix/perf select patch, and other types do not release alone. Read all first-parent messages since the previous tag so coalescing preserves the highest requested bump.
friction: Hosted CI has no Lefthook. The new commit rejection test initially assumed the local tool existed; it now exercises the identical command through a direct Git hook when Lefthook is unavailable, retaining real commit rejection coverage on CI.
