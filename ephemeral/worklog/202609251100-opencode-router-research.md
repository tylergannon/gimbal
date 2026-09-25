decision: Target installed OpenCode 1.2.27 with singular provider config and CLI turn adapter first; treat published v2 configuration and server API as a separate migration.
doc_bug: Diffusion Router portal OpenCode guide mixes v1 provider and v2 providers keys -> installed OpenCode rejects it with Unrecognized key providers; use one singular provider.diffusion object with npm, options, and models.
decision: OpenCode custom provider model listing reflects configured models; compare against the Diffusion catalog separately and avoid treating opencode models as full gateway discovery.
decision: OPENCODE_CONFIG_CONTENT is a merged override rather than a clean profile; use process scoped env for router configuration and test stricter XDG isolation if required.
