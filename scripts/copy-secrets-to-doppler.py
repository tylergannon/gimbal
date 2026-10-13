#!/usr/bin/env python3
"""Copy one fixed GitHub environment secret group into gimbal/stg."""

import json
import os
import sys
import urllib.error
import urllib.request

GROUPS = {
    "preparation": ("RELEASE_TAG_SSH_KEY",),
    "release": (
        "DEVELOPER_ID_CERTIFICATE_P12_BASE64",
        "DEVELOPER_ID_CERTIFICATE_PASSWORD",
        "APPLE_TEAM_ID",
        "APP_STORE_CONNECT_API_KEY_P8_BASE64",
        "APP_STORE_CONNECT_KEY_ID",
        "APP_STORE_CONNECT_ISSUER_ID",
        "SPARKLE_ED_PRIVATE_KEY",
        "TAP_DEPLOY_KEY",
    ),
}
DOWNLOAD = (
    "https://api.doppler.com/v3/configs/config/secrets/download"
    "?project=gimbal&config=stg&format=json"
)
UPDATE = "https://api.doppler.com/v3/configs/config/secrets"


def request(token, url, payload=None):
    data = None if payload is None else json.dumps(payload).encode()
    req = urllib.request.Request(
        url, data=data,
        headers={"Authorization": "Bearer " + token,
                 "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            # Never print responses: the API can return secret values.
            return json.load(response)
    except urllib.error.HTTPError as error:
        raise RuntimeError(f"Doppler request failed: HTTP {error.code}") from None
    except Exception:
        raise RuntimeError("Doppler request failed; response suppressed") from None


def copy(group, env=os.environ):
    names = GROUPS[group]
    if not env.get("DOPPLER_IMPORT_TOKEN"):
        raise RuntimeError("DOPPLER_TOKEN_RW is missing")
    missing = [name for name in names if not env.get(name)]
    if missing:
        raise RuntimeError("Missing source secrets: " + ", ".join(missing))
    values = {name: env[name] for name in names}
    token = env["DOPPLER_IMPORT_TOKEN"]
    before = request(token, DOWNLOAD)
    if before.get("DOPPLER_PROJECT") != "gimbal" or before.get("DOPPLER_CONFIG") != "stg":
        raise RuntimeError("Doppler destination is not gimbal/stg")
    conflicts = [name for name in names if name in before and before[name] != values[name]]
    if conflicts:
        raise RuntimeError("Destination already has different values: " + ", ".join(conflicts))
    request(token, UPDATE, {"project": "gimbal", "config": "stg", "secrets": values})
    after = request(token, DOWNLOAD)
    failed = [name for name in names if after.get(name) != values[name]]
    if failed:
        raise RuntimeError("Copy verification failed: " + ", ".join(failed))
    for name in names:
        print(f"Copied and verified: {name}")
    print(f"Verified {len(names)} secrets in gimbal/stg")


if __name__ == "__main__":
    if len(sys.argv) != 2 or sys.argv[1] not in GROUPS:
        sys.exit("Usage: copy-secrets-to-doppler.py preparation|release")
    try:
        copy(sys.argv[1])
    except RuntimeError as error:
        sys.exit(str(error))
