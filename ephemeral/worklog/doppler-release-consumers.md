# Doppler-backed release consumers

decision: User removed the old deploykey and requested immediate workflow cutover to Doppler, with GitHub environments removed unless valuable. Retain main/tag-only environment guards but store only the existing read-only gimbal/stg service token in each; remove repository-wide token access and legacy credential copies after proving the new release path. Do not create new credentials.

decision: Use the pinned official Doppler fetch action with masked per-step outputs and a config identity check. Replace all nine credential references in tagging/signing/Homebrew jobs; guard model refresh under the main-only environment, since its bootstrap token now reads the release config too. Update official key identity and rotation instructions; retire the completed import workflow.

friction: Credential-only PRs do not need full application CI; validate workflow wiring and existing automation tests locally. One normal main release run establishes actual tag/sign/notarize/tap consumption. No application UI changes or local rebuild/installation required.
