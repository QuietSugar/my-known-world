#!/usr/bin/env python3
"""Compares a source MyKnownWorld instance with a restored instance to verify backup/restore fidelity."""
import argparse
import json
import sys
import urllib.error
import urllib.request


def fetch_json(url, path, token=None):
    headers = {"Accept": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(url.rstrip("/") + path, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return resp.status, json.load(resp)
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read().decode())
        except Exception:
            return e.code, {}


def main():
    parser = argparse.ArgumentParser(description="Verify backup/restore fidelity between two instances")
    parser.add_argument("--source-url", required=True, help="Base URL of source instance")
    parser.add_argument("--restored-url", required=True, help="Base URL of restored instance")
    parser.add_argument("--cmdb-token", help="Optional CMDB app token to verify credential decryption")
    parser.add_argument("--credential-ref", help="Optional credential opaqueId to verify")
    parser.add_argument("--expected-secret", help="Expected decrypted credential value")
    args = parser.parse_args()

    print(f"==> Verifying source ({args.source_url}) vs restored ({args.restored_url})...")

    # 1. Verify health
    s_code, s_health = fetch_json(args.source_url, "/healthz")
    r_code, r_health = fetch_json(args.restored_url, "/healthz")
    if s_code != 200 or r_code != 200:
        sys.exit(f"Healthcheck failed: source={s_code}, restored={r_code}")
    print("✓ Both instances healthy")

    # 2. Fetch and compare nodes
    s_code, s_nodes_resp = fetch_json(args.source_url, "/api/nodes")
    r_code, r_nodes_resp = fetch_json(args.restored_url, "/api/nodes")
    if s_code != 200 or r_code != 200:
        sys.exit(f"Failed to fetch nodes: source={s_code}, restored={r_code}")

    s_nodes = s_nodes_resp.get("items", [])
    r_nodes = r_nodes_resp.get("items", [])
    if len(s_nodes) != len(r_nodes):
        sys.exit(f"Node count mismatch: source={len(s_nodes)}, restored={len(r_nodes)}")

    s_nodes_sorted = sorted(s_nodes, key=lambda n: n.get("id", ""))
    r_nodes_sorted = sorted(r_nodes, key=lambda n: n.get("id", ""))
    for sn, rn in zip(s_nodes_sorted, r_nodes_sorted):
        if sn.get("id") != rn.get("id") or sn.get("type") != rn.get("type") or sn.get("name") != rn.get("name"):
            sys.exit(f"Node mismatch: source={sn} vs restored={rn}")
    print(f"✓ Nodes match identically ({len(s_nodes)} nodes)")

    # 3. Fetch and compare relations
    s_code, s_rels_resp = fetch_json(args.source_url, "/api/relations")
    r_code, r_rels_resp = fetch_json(args.restored_url, "/api/relations")
    if s_code != 200 or r_code != 200:
        sys.exit(f"Failed to fetch relations: source={s_code}, restored={r_code}")

    s_rels = s_rels_resp.get("items", [])
    r_rels = r_rels_resp.get("items", [])
    if len(s_rels) != len(r_rels):
        sys.exit(f"Relation count mismatch: source={len(s_rels)}, restored={len(r_rels)}")

    s_rels_sorted = sorted(s_rels, key=lambda r: (r.get("id", ""), r.get("from", ""), r.get("predicate", ""), r.get("to", "")))
    r_rels_sorted = sorted(r_rels, key=lambda r: (r.get("id", ""), r.get("from", ""), r.get("predicate", ""), r.get("to", "")))
    for sr, rr in zip(s_rels_sorted, r_rels_sorted):
        if (sr.get("id") != rr.get("id") or
            sr.get("from") != rr.get("from") or
            sr.get("predicate") != rr.get("predicate") or
            sr.get("to") != rr.get("to")):
            sys.exit(f"Relation mismatch: source={sr} vs restored={rr}")
    print(f"✓ Relations match identically ({len(s_rels)} relations)")

    # 4. Optional CMDB Credential Readback check
    if args.cmdb_token and args.credential_ref and args.expected_secret:
        c_code, cred = fetch_json(args.restored_url, f"/cmdb/credentials/{args.credential_ref}", token=args.cmdb_token)
        if c_code != 200 or cred.get("value") != args.expected_secret:
            sys.exit(f"Restored CMDB credential decryption failed: code={c_code}, value={cred.get('value')}")
        print("✓ CMDB encrypted credential correctly restored and decrypted with matching vault key")

    print("==> All restore fidelity verification checks PASSED!")


if __name__ == "__main__":
    main()
