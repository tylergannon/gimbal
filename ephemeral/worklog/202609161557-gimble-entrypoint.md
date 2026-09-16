decision: Move the CLI package to cmd/gimble so Go derives the binary name from the package directory, as Tyler requested.
friction: GIMBLE108 exempts the CLI by exact import path; a command relocation must move that exemption and its analysistest fixture along with build, hook, and generation references.
friction: The commit hook rejects the pre-existing generation-only roles maps as unused. Generated command defaults now read their role map entries directly, preserving the configured defaults while making the declarations used.
correction: Tyler wants WorkDir instead of Repo for the workflow working directory; apply the name consistently to inputs, CLI flags, schema, generated commands, and documentation.
