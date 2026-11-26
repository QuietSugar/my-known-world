#!/usr/bin/env python3
"""比对 hub 的 frontmatter 与数据库，报告哪些事实还没入库。

权威数据源是 my-known-world，hub 是投影层。搬迁是否完成，取决于库能否回答
hub 曾经承载的全部结构化事实。本脚本逐实体逐字段比对，输出缺口清单。

用法：
  python3 scripts/parity-report.py            # 报告
  python3 scripts/parity-report.py --strict   # 有缺口时返回非零
"""
import argparse
import json
import os
import re
import subprocess
import sys

# frontmatter 键 -> 在库中的位置。未列出的键（正文、notes 等）是叙述性内容，
# 不要求入库，也不计入比对。
FIELD_MAP = {
    "hardware": {
        "model": "details.model",
        "location": "details.location",
        "status": "details.status",
        "mac": "details.macs",
        "specifications": "details.specifications",
        "category": "details.category",
    },
    "rig": {
        "hostname": "details.hostname",
        "ip": "details.ips",
        "os": "details.os",
        "status": "details.status",
        "spec": "details.spec",
        "role": "details.roles",
        "constraints": "details.operationConstraints",
        "owned": "details.owned",
        "access_method": "details.accessMethod",
        "access_prerequisites": "details.accessPrerequisites",
        "responsibility_boundary": "details.responsibilityBoundary",
        "allowed_operations": "details.allowedOperations",
        "forbidden_operations": "details.forbiddenOperations",
    },
    "service": {
        "name": "node.name",
        "host": "rel.hosts",
        "parent": "rel.hosts",
        "status": "node.lifecycle",
        "deploy_path": "details.deploymentPath",
        "deployment_path": "details.deploymentPath",
        "container": "details.runtimeIdentity",
        "ports": "details.ports",
        "domain": "details.domains",
        "domains": "details.domains",
        "deployment_method": "details.deploymentMethod",
        "maintenance_state": "details.maintenanceState",
    },
    "product": {
        "name": "node.name",
        "status": "node.lifecycle",
        "purpose": "details.purpose",
        "runbook_reference": "details.runbookReference",
        "common_configuration_reference": "details.commonConfigurationReference",
    },
    "design": {
        "name": "node.name",
        "status": "node.lifecycle",
        "role": "details.role",
        "authoritative_reference": "details.authoritativeReference",
        "effective": "details.effective",
    },
    "runbook": {
        "name": "node.name",
        "status": "node.lifecycle",
        "scope": "details.scope",
        "trigger": "details.trigger",
        "steps_reference": "details.stepsReference",
        "rollback_reference": "details.rollbackReference",
        "verification_method": "details.verificationMethod",
        "preconditions": "details.preconditions",
    },
}
FIELD_MAP["shared"] = FIELD_MAP["rig"]

DETAIL_TABLES = ["hardware_details", "machine_details", "software_instance_details",
                 "product_details", "document_details", "runbook_details",
                 "software_project_details", "account_credential_details",
                 "network_details", "work_item_details"]


def load_mapping(config_path, wiki_root):
    mapping = {}
    source_id = None
    with open(config_path, encoding="utf-8") as fh:
        for line in fh:
            m = re.match(r"\s*-\s*sourceId:\s*(\S+)", line)
            if m:
                source_id = m.group(1)
                continue
            m = re.match(r"\s*nodeId:\s*([0-9a-f-]{36})", line)
            if m and source_id:
                if os.path.exists(os.path.join(wiki_root, source_id)):
                    mapping[source_id] = m.group(1)
                source_id = None
    return mapping


def read_frontmatter(path):
    text = open(path, encoding="utf-8").read()
    m = re.match(r"^---\n(.*?)\n---\n", text, re.S)
    if not m:
        return None
    data, key = {}, None
    for line in m.group(1).split("\n"):
        if re.match(r"^\s+", line) and key:
            item = line.strip()
            if item.startswith("- "):
                if not isinstance(data.get(key), list):
                    data[key] = []
                data[key].append(item[2:].strip().strip('"'))
            elif ":" in item:
                if not isinstance(data.get(key), dict):
                    data[key] = {}
                k, _, v = item.partition(":")
                data[key][k.strip()] = v.strip().strip('"')
            continue
        km = re.match(r"^([a-z_]+):\s*(.*)$", line)
        if km:
            key, value = km.group(1), km.group(2).strip()
            data[key] = value.strip('"') if value else None
    return data


def gather_db(psql):
    """一次查询取回所有节点的基础字段、细节 JSON 与 hosts 关系存在性。"""
    union = " UNION ALL ".join(
        f"SELECT node_id, '{t}' AS tbl, details::text AS d FROM {t}" for t in DETAIL_TABLES)
    sql = (f"SELECT n.id, n.name, coalesce(n.lifecycle,''), "
           f"coalesce((SELECT 1 FROM relations r WHERE r.to_id=n.id AND r.predicate='hosts' LIMIT 1),0), "
           f"coalesce(x.tbl,''), coalesce(x.d,'') "
           f"FROM nodes n LEFT JOIN ({union}) x ON x.node_id = n.id")
    # SQL 走 stdin：查询里含单引号，放进 -c 会被本地 shell 拆词。
    out = subprocess.run(psql, shell=True, input=sql,
                         capture_output=True, text=True).stdout
    db = {}
    for line in out.split("\n"):
        if not line.strip():
            continue
        parts = line.split("|", 5)
        if len(parts) < 6:
            continue
        node_id, name, lifecycle, has_hosts, _tbl, details = parts
        entry = db.setdefault(node_id, {"node": {"name": name, "lifecycle": lifecycle},
                                        "rel": {"hosts": has_hosts == "1"},
                                        "details": {}})
        if details:
            try:
                entry["details"] = json.loads(details)
            except json.JSONDecodeError:
                pass
    return db


def present(entry, path):
    parts = path.split(".")
    cur = entry.get(parts[0], {})
    for part in parts[1:]:
        if not isinstance(cur, dict):
            return False
        cur = cur.get(part)
    return cur not in (None, "", [], {})


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--wiki-root", default=os.environ.get(
        "HUB_WIKI",
        "/home/dev/git-repo/codeup.aliyun.com/5ed719afd1d1abe63b55e7da/hub/hall-wiki"))
    ap.add_argument("--config", default=os.environ.get(
        "WIKI_SOURCE", "/home/dev/deploy/my-known-world/wiki-source.yaml"))
    ap.add_argument("--psql", default=(
        'ssh svc "docker exec -i hall-pgsql psql -U myknownworld -d myknownworld -tA"'))
    ap.add_argument("--strict", action="store_true")
    args = ap.parse_args()

    mapping = load_mapping(args.config, args.wiki_root)
    db = gather_db(args.psql)

    checked, gaps = 0, []
    for rel, node_id in sorted(mapping.items()):
        fm = read_frontmatter(os.path.join(args.wiki_root, rel))
        if not fm:
            continue
        fields = FIELD_MAP.get((fm.get("type") or "").lower())
        if not fields:
            continue
        entry = db.get(node_id)
        if not entry:
            gaps.append((rel, "(node)", "节点不在库中"))
            continue
        for key, target in fields.items():
            if fm.get(key) in (None, "", [], {}):
                continue
            checked += 1
            if not present(entry, target):
                gaps.append((rel, key, target))

    print(f"比对 {checked} 个字段，缺口 {len(gaps)}")
    by_file = {}
    for rel, key, target in gaps:
        by_file.setdefault(rel, []).append(f"{key} -> {target}")
    for rel, items in sorted(by_file.items()):
        print(f"\n{rel}")
        for item in items:
            print(f"    {item}")
    return 1 if (args.strict and gaps) else 0


if __name__ == "__main__":
    sys.exit(main())
