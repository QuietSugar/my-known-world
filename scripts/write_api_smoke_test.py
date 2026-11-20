#!/usr/bin/env python3
"""Disposable synthetic checks for the create API against a running deployment."""
import argparse
import json
import urllib.error
import urllib.request


def call(base, path, method="GET", token=None, body=None):
    data = None if body is None else json.dumps(body).encode()
    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(base.rstrip("/") + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as exc:
        return exc.code, {}


def require(condition, message):
    if not condition:
        raise SystemExit(message)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--token", required=True)
    args = parser.parse_args()

    nodes = [
        {"node": {"id": "ci-write-machine", "type": "Machine", "name": "CI write machine", "modelVersion": 1, "source": "ci:write-api"}},
        {"node": {"id": "ci-write-project", "type": "SoftwareProject", "name": "CI write project", "modelVersion": 1, "source": "ci:write-api"}, "anchor": {"predicate": "maintains"}},
    ]
    relations = [{"id": "ci-write-depends", "from": "ci-write-project", "predicate": "dependsOn", "to": "ci-write-machine", "source": "ci:write-api"}]

    status, unauthorized = call(args.base_url, "/api/write/batch", "POST", None, {"nodes": nodes})
    require(status == 401, f"create without a token must fail, got {status}")

    status, created = call(args.base_url, "/api/write/batch", "POST", args.token, {"nodes": nodes, "relations": relations})
    require(status == 201, f"batch create failed with {status}: {created}")
    body = created.get("created", {})
    require(body.get("nodeIds") == ["ci-write-machine", "ci-write-project"], f"unexpected node ids: {body}")
    require(body.get("relationIds") == ["anchor-ci-write-machine", "anchor-ci-write-project", "ci-write-depends"], f"unexpected relation ids: {body}")
    require(created.get("revision"), "create response did not include a revision")

    status, node = call(args.base_url, "/api/nodes/ci-write-machine")
    require(status == 200 and node.get("name") == "CI write machine", f"created node is not readable: {status} {node}")

    status, _ = call(args.base_url, "/api/write/nodes", "POST", args.token, {"node": {"id": "ci-write-machine", "type": "Machine", "name": "duplicate", "modelVersion": 1}})
    require(status == 409, f"duplicate node must be rejected, got {status}")

    status, single = call(args.base_url, "/api/write/relations", "POST", args.token, {"relation": {"id": "ci-write-ghost", "from": "ci-write-project", "predicate": "dependsOn", "to": "missing-node"}})
    require(status == 409, f"dangling relation must be rejected, got {status}")

    status, stale = call(
        args.base_url,
        "/api/write/nodes",
        "POST",
        args.token,
        {"expectedRevision": "not-the-current-revision", "node": {"id": "ci-write-stale", "type": "Machine", "name": "stale", "modelVersion": 1}},
    )
    require(status == 409, f"stale revision must be rejected, got {status}")

    # --- PATCH & DELETE DIRECT MUTATIONS (Issue #47) ---
    rev = created.get("revision")

    # PATCH node with stale revision fails
    status, _ = call(
        args.base_url,
        "/api/write/nodes/ci-write-machine",
        "PATCH",
        args.token,
        {"expectedRevision": "wrong-revision", "node": {"name": "Patched name"}},
    )
    require(status == 409, f"stale revision patch must fail with 409, got {status}")

    # PATCH node succeeds
    status, patched = call(
        args.base_url,
        "/api/write/nodes/ci-write-machine",
        "PATCH",
        args.token,
        {"expectedRevision": rev, "node": {"name": "Patched name"}},
    )
    require(status == 200, f"patch node failed with {status}: {patched}")
    require(patched.get("node", {}).get("name") == "Patched name", f"node was not patched: {patched}")
    rev = patched.get("revision")

    # Read back to ensure update was persisted
    status, read_back = call(args.base_url, "/api/nodes/ci-write-machine")
    require(status == 200 and read_back.get("name") == "Patched name", f"patched node not readable: {read_back}")

    # PATCH relation succeeds
    status, patched_rel = call(
        args.base_url,
        "/api/write/relations/ci-write-depends",
        "PATCH",
        args.token,
        {"expectedRevision": rev, "relation": {"certainty": "confirmed"}},
    )
    require(status == 200, f"patch relation failed with {status}: {patched_rel}")
    require(patched_rel.get("relation", {}).get("certainty") == "confirmed", f"relation was not patched: {patched_rel}")
    rev = patched_rel.get("revision")

    # DELETE node blocked by active relation
    status, _ = call(
        args.base_url,
        "/api/write/nodes/ci-write-machine",
        "DELETE",
        args.token,
        {"expectedRevision": rev},
    )
    require(status == 409, f"deleting node with active relation must return 409, got {status}")

    # DELETE relation succeeds
    status, deleted_rel = call(
        args.base_url,
        "/api/write/relations/ci-write-depends",
        "DELETE",
        args.token,
        {"expectedRevision": rev},
    )
    require(status == 200, f"delete relation failed with {status}: {deleted_rel}")
    rev = deleted_rel.get("revision")

    # DELETE node now succeeds
    status, deleted_proj = call(
        args.base_url,
        "/api/write/nodes/ci-write-project",
        "DELETE",
        args.token,
        {"expectedRevision": rev},
    )
    require(status == 200, f"delete project node failed with {status}: {deleted_proj}")
    rev = deleted_proj.get("revision")

    status, deleted_mach = call(
        args.base_url,
        "/api/write/nodes/ci-write-machine",
        "DELETE",
        args.token,
        {"expectedRevision": rev},
    )
    require(status == 200, f"delete machine node failed with {status}: {deleted_mach}")

    # Read back returns 404
    status, _ = call(args.base_url, "/api/nodes/ci-write-machine")
    require(status == 404, f"deleted node should return 404, got {status}")
    status, _ = call(args.base_url, "/api/nodes/ci-write-project")
    require(status == 404, f"deleted node should return 404, got {status}")

    print("write API checks passed (auth, batch create, read-back, duplicate, dangling relation, stale revision, patch node/relation, delete integrity guard, delete cleanup, 404 read-back)")


if __name__ == "__main__":
    main()
