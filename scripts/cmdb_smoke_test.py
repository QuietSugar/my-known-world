#!/usr/bin/env python3
"""Disposable synthetic credential lifecycle checks for the CMDB test stack."""
import argparse
import json
import urllib.error
import urllib.request


def call(base, token, path, method="GET", body=None):
    data = None if body is None else json.dumps(body).encode()
    request = urllib.request.Request(
        base.rstrip("/") + path,
        data=data,
        method=method,
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json", "Accept": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as exc:
        raise RuntimeError(f"CMDB smoke request failed ({method} {path}: HTTP {exc.code})") from None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--token", required=True)
    args = parser.parse_args()
    status, created = call(args.base_url, args.token, "/cmdb/credentials", "POST", {"name": "ci-synthetic-credential", "value": "synthetic-only-value-v0"})
    ref = created.get("metadata", {}).get("opaqueId")
    if status != 201 or not ref or "value" in created:
        raise SystemExit("CMDB create did not return metadata-only success")
    status, value = call(args.base_url, args.token, f"/cmdb/credentials/{ref}")
    if status != 200 or value.get("value") != "synthetic-only-value-v0":
        raise SystemExit("CMDB read lifecycle check failed")
    call(args.base_url, args.token, f"/cmdb/credentials/{ref}", "PUT", {"value": "synthetic-only-value-v1"})
    call(args.base_url, args.token, f"/cmdb/credentials/{ref}/rotate", "POST", {"value": "synthetic-only-value-v2"})
    call(args.base_url, args.token, f"/cmdb/credentials/{ref}/disable", "POST")
    try:
        call(args.base_url, args.token, f"/cmdb/credentials/{ref}")
    except RuntimeError as exc:
        if "HTTP 404" in str(exc):
            print("CMDB synthetic credential lifecycle checks passed (create/read/update/rotate/disable)")
            return
        raise
    raise SystemExit("Disabled synthetic credential remained readable")


if __name__ == "__main__":
    main()
