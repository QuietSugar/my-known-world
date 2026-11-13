#!/usr/bin/env python3
"""Read-only smoke test for a running MyKnownWorld API deployment."""

from __future__ import annotations

import argparse
import json
import sys
import urllib.error
import urllib.parse
import urllib.request

ALLOWED_EVIDENCE = {"unknown", "manual", "document", "observed", "derived", "imported"}
ALLOWED_CERTAINTY = {"unknown", "provisional", "confirmed", "disputed", "stale"}
NON_IMPACT_RELATIONS = {"knowsAbout", "documents", "appliesTo"}


def get_json(base: str, path: str):
    request = urllib.request.Request(
        f"{base.rstrip('/')}{path}", headers={"Accept": "application/json"}
    )
    try:
        with urllib.request.urlopen(request, timeout=5) as response:
            if response.status != 200:
                raise AssertionError(f"GET {path}: expected HTTP 200, got {response.status}")
            return json.load(response)
    except urllib.error.URLError as exc:
        raise AssertionError(f"GET {path}: {exc}") from exc


def check(base: str) -> int:
    assert get_json(base, "/healthz").get("status") == "ok", "health check is not ok"
    nodes = get_json(base, "/api/nodes").get("items")
    assert isinstance(nodes, list), "nodes response must include an items array"
    self_nodes = [node for node in nodes if node.get("type") == "Self"]
    assert len(self_nodes) == 1, f"expected exactly one Self node, found {len(self_nodes)}"
    self_id = self_nodes[0]["id"]

    known_ids = {node["id"] for node in nodes}
    assert len(known_ids) == len(nodes), "node IDs must be unique"
    for node in nodes:
        assert node.get("source"), f"node {node['id']} has no source state"
        assert node.get("evidenceKind") in ALLOWED_EVIDENCE, f"invalid evidenceKind on {node['id']}"
        assert node.get("certainty") in ALLOWED_CERTAINTY, f"invalid certainty on {node['id']}"
        path = get_json(base, f"/api/path/{urllib.parse.quote(node['id'], safe='')}")
        path_nodes = path.get("nodes", [])
        relations = path.get("relations", [])
        assert path_nodes and path_nodes[0]["id"] == self_id, f"{node['id']} path must start at Self"
        assert path_nodes[-1]["id"] == node["id"], f"{node['id']} path must end at that node"
        assert len(relations) == max(0, len(path_nodes) - 1), f"broken relation chain for {node['id']}"
        for index, relation in enumerate(relations):
            pair = {path_nodes[index]["id"], path_nodes[index + 1]["id"]}
            assert {relation["from"], relation["to"]} == pair, f"relation does not join adjacent path nodes: {relation['id']}"

    impact = get_json(base, "/api/impact").get("items")
    assert isinstance(impact, list), "impact response must include an items array"
    for item in impact:
        target = item["node"]["id"]
        assert target in known_ids, f"impact references unknown node {target}"
        relations = item.get("relationPath", [])
        assert relations, f"impact path for {target} must include its relations"
        assert all(rel.get("predicate") not in NON_IMPACT_RELATIONS for rel in relations), f"non-impact relation leaked into impact path for {target}"
        assert relations[0]["from"] == self_id, f"impact path for {target} must begin at Self"
        assert relations[-1]["to"] == target, f"impact path for {target} must end at target"
        for previous, current in zip(relations, relations[1:]):
            assert previous["to"] == current["from"], f"broken impact relation chain for {target}"

    print(f"PASS: health, {len(nodes)} nodes, Self connectivity, provenance fields, {len(impact)} impact paths")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:8080", help="API base URL")
    args = parser.parse_args()
    try:
        return check(args.base_url)
    except (AssertionError, KeyError, TypeError, ValueError) as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
