# Adversarial review: release diagnosis, round 02

## Target

Revised `ephemeral/research/release-diagnosis/diagnosis.md`, judged against
the request it records (why release workflows failed, why Python entered, how
proof missed failures, what is broken now, double-check everything),
`AGENTS.md`, `scripts/release_timing.py`, the workflows, and live state.

The launch prompt supplied only target, evidence locations, artifact path, and
read-only limits. These are valid operating constraints; none narrowed scope.

## Evidence inspected and independently re-run

Published-artifact claims, all verified by me in this review:

- Release v0.13.23: draft=false, prerelease=false, seven assets. GitHub asset
  sizes and digests equal the SHA-256 of the files in
  `/tmp/gimbal-release-audit-assets` (shasum). Confirmed.
- `checksums.txt` lists five packages (three CLI archives, ZIP, DMG); each hash
  equals the downloaded file. Confirmed.
- `releases/latest/download/appcast.xml` fetched live is byte-identical to the
  downloaded appcast. Enclosure URL and length 25832061 match the ZIP;
  version 27, short version 0.13.23, minimum system 27.0. Previous appcast
  carries `sparkle:version` 26. Confirmed.
- Appcast edSignature verified over the exact ZIP bytes with OpenSSL Ed25519
  (raw) against `SUPublicEDKey` from the unpacked app's Info.plist:
  "Signature Verified Successfully". Confirmed, by a different tool than the
  diagnosis used.
- Unpacked app: `codesign --verify --deep --strict` passes; `stapler validate`
  works; `spctl -a -vv` = accepted, Notarized Developer ID. Info.plist reports
  0.13.23 / 27 / 27.0. Confirmed.
- DMG: `hdiutil verify` VALID; `codesign --verify --strict` valid and
  satisfies its Designated Requirement; `stapler validate` works. Confirmed.
- Homebrew: live `tylergannon/homebrew-gimbal` `Casks/gimbal.rb` and
  `gimbal-app.rb` both have version 0.13.23 and sha256 equal to the Darwin
  CLI archive and DMG. The release workflow (`release.yml:295`) confirms that
  repository is the tap. Confirmed.

Monitor claims:

- Same facts as round 01 re-checked: job 115944920729 reversed timestamps,
  scan aborts with exactly that message, later hourly steps skipped, HTTP 504
  in run 38605329199.
- The revised counts: since 15:36Z the workflow shows 37 failures and 1
  cancellation in the sampled history, matching "37 failures, one
  cancellation, one success before the failure began" (39 sampled).
  The correction that these are not 37 consecutive failures is right.
- `ensure_issue` (`release_timing.py:183-207`) only searches for a marker and
  creates; it never updates or closes. The diagnosis's rejection of my
  round-01 remark that the broken observer "cannot update or close" issues
  is correct; that remark was mine and was wrong. The adjudication is accurate.

Not re-run: the standalone and bundled CLI start-up probes, the
"archived-plugin/web package checks" log claims, and Linux binaries. The
diagnosis labels these as not freshly proved or log-derived, which is honest.

## Findings

No critical findings and no issues. The revision incorporates round 01
accurately, fixes the overstatement about consecutive failures, and adds
appropriately scoped caveats about the standalone CLI and spctl.

### 1. nitpick: review section is now partially circular

`## Independent review` describes this round as "pending". When this review is
accepted, that sentence should be replaced with this round's outcome, and the
section should state that artifact signing/integrity claims were verified a
second time here with different tooling (OpenSSL for the Ed25519 signature,
hdiutil/spctl/stapler/codesign) and that the CLI probes and Linux runtime
evidence were not repeated.

### 2. nitpick: "Checksums file is not an independent authority"

Correctly stated, but note that the Homebrew casks and GitHub asset digests
are separate publication channels that independently agree with the
downloaded bytes; saying so would strengthen the consistency claim without
overreaching.

## Outcome

only nitpicks remain
