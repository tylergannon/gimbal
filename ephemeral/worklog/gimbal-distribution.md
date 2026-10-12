# Gimbal distribution

decision: User authorized Homebrew CLI publication, Linux binary downloads, and signed macOS application distribution, with four independent Luna research agents and local build/sign proof before automation.
decision: Preserve the current desktop shell and independent server boundary; packaging does not authorize introducing an app-owned server lifecycle.
friction: Apple Developer account is logged in but membership expired; local Keychain contains no valid signing identity. Renewal requested directly from user before signing work.
correction: The standard CLI imports Go plugin; CGO_ENABLED=0 produces a restricted binary and must not be the standard published artifact.
doc_bug: Installed release skill still describes committed web/build.zip, while maintained skill and current source embed generated web/build directly. Use maintained source and actual build recipe.
correction: User had just renewed Apple membership; refreshed account confirms renewal through October 12, 2027. Certificate API still rejected the same team, so this is an access-propagation/service issue, not an unpaid renewal.
friction: Safari's API-key download permission dialog includes a private-key data URL in its accessibility text. Returning an unfiltered AX state exposed it to the tool transcript. Download was cancelled, key will not be used, and user was asked to revoke it and download a replacement manually. Never emit complete credential-dialog AX states; use file-based handling without opening private key content.
friction: actionlint 1.7.12 lacks the officially supported xcode-27 preview runner label. Removed a draft linter suppression config to keep the repository's default-linter policy; report the label-catalog limitation instead of hiding it.
decision: Apple certificate access is now restored. The public Developer ID Application CSR is uploaded with the G2 issuer selected; final certificate issuance and notarization-key replacement remain with the user under the computer-use credential policy.
friction: Safari's upload Go To dialog retained its previous folder text. Setting the path field explicitly and verifying its value before Return selected the CSR correctly; avoid blind paste into that dialog.
