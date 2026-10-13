# One-time Doppler import

decision: User explicitly authorized copying the nine existing GitHub environment secrets into gimbal/stg using the repository DOPPLER_TOKEN_RW, which expires after 30 minutes. Do not recreate credentials, delete originals, change publishing, or copy the bootstrap token.

decision: Preserve main-only release-preparation and tag-only release environment policies. A manually dispatched fixed-list importer writes only the selected group and verifies equality in memory; suppress all API response bodies and secret values. A migration-only tag is ignored by stable patch-version discovery and starts no signed release pipeline.

friction: Automatic release currently triggers full application checks for every workflow/script PR and main push. This credential-copy task uses focused importer tests and actionlint; cancel only its own redundant automatic runs. No application changes or new signing release are needed.
