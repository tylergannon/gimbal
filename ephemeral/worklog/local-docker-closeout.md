# Local and Docker delivery closeout

- correction: Tyler rejected committing the frontend ZIP and requested regular embed.FS. Embed the generated build directory directly, remove ZIP packaging, and keep generated assets local. Local installation follows just build.
- skill_issue: gimbal-release source=skills/gimbal-release/SKILL.md severity=bug -> The committed-archive and bare versioned-install instructions no longer apply. Update maintained instructions before refreshing the local skills.
- decision: Finish the accepted local/Docker slice in PR #444. Observation message-bus separation remains the next delivery; effect transport is considered separately.
