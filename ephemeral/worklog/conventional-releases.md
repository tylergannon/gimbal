# Conventional releases

decision: Tyler requested Conventional Commit version selection and rejection of breaking-change declarations in the local commit hook. Gimbal is pre-release with no users; major version increments are forbidden. Classify pre-release API changes as features or fixes.
correction: Automatic publication introduced on 2026-10-13 hardcoded patch bumps; automation must preserve meaningful version selection.
decision: feat selects minor, fix/perf select patch, and other types do not release alone. Read all first-parent messages since the previous tag so coalescing preserves the highest requested bump.
