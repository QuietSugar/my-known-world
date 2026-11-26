#!/usr/bin/env python3
"""逐值比对 hub 的 frontmatter 与数据库，证明搬迁保真。

my-known-world 是权威数据源，hub 是投影层。本脚本回答两个问题：

  1. hub 里的每一条 frontmatter 事实，库里是否有**相同的值**（不只是有字段）。
  2. hub 正文里剩余的内容，是已入库的重复，还是明确保留的叙述。

字段集不靠手写白名单：导入器把任意 snake_case frontmatter 键转成 camelCase
后按模型解码，所以可接受字段来自 internal/graph/models.go 的 JSON tag。脚本
直接解析该文件生成对照表，模型加字段时无需改本脚本。

用法：
  python3 scripts/parity-report.py             # 报告缺口
  python3 scripts/parity-report.py --strict    # 有缺口时退出码 1
  python3 scripts/parity-report.py --values    # 同时显示值不等的明细
"""
import argparse
import json
import os
import re
import subprocess
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MODELS_GO = os.path.join(REPO_ROOT, "internal", "graph", "models.go")

# 源类型 -> 模型结构体名。与 importer 的 node.Type 分支对应。
TYPE_TO_MODEL = {
    "hardware": "HardwareDetails",
    "rig": "MachineDetails",
    "shared": "MachineDetails",
    "service": "SoftwareInstanceDetails",
    "product": "ProductDetails",
    "design": "DocumentDetails",
    "runbook": "RunbookDetails",
    "task": "WorkItemDetails",
    "network": "NetworkDetails",
}

# frontmatter 键与模型字段名不一致的，importer 里有显式别名。
KEY_ALIASES = {
    "container": "runtimeIdentity",
    "recovery_assets": "configurationReferences",
    "deploy_path": "deploymentPath",
    "ip": "ips",
    "mac": "macs",
    # role 只在 machine 上表示多值岗位；document 的 role 就是单数 role，
    # 因此别名按模型区分，见 alias_for()。
    "constraints": "operationConstraints",
    "domain": "domains",
}

# 记录元数据，不是业务事实：type 决定映射到哪个模型，id 已作为节点主键。
# title 是 name 的别名，模型的 name 存在节点列而不是 details。
# title 是人类可读的展示标题，与作为标识的 name 并存，不是遗漏。
META_KEYS = {"type", "id", "title"}

# 由关系或节点列承载的键，不在 details JSON 里。
NODE_LEVEL = {"id": "id", "name": "name", "status": "lifecycle"}
RELATION_LEVEL = {"host": "hosts", "parent": "hosts"}


def alias_for(key, model_name, allowed):
    """frontmatter 键到模型字段的映射。

    多数别名是固定的（container -> runtimeIdentity），但 role 这类键在
    machine 上是 roles（多值），在 document 上是 role（单值）。按模型里实际
    存在的字段决定，避免把单值映射成不存在的复数形式。
    """
    if key == "role":
        if "role" in allowed:
            return "role"
        if "roles" in allowed:
            return "roles"
        return "role"
    field = KEY_ALIASES.get(key)
    if field:
        return field
    return re.sub(r"_(\w)", lambda m: m.group(1).upper(), key)


def load_model_fields():
    """解析 models.go，得到 结构体名 -> {json 字段名}。"""
    src = open(MODELS_GO, encoding="utf-8").read()
    out = {}
    for name, body in re.findall(r"type (\w+Details) struct \{(.*?)\n\}", src, re.S):
        out[name] = set(re.findall(r'json:"([a-zA-Z]+)', body))
    return out


def load_mapping(config_path, wiki_root):
    mapping, source_id = {}, None
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
    m = re.match(r"^---\n(.*?)\n---\n(.*)$", text, re.S)
    if not m:
        return None, text
    data, key = {}, None
    for line in m.group(1).split("\n"):
        if re.match(r"^\s+", line) and key:
            item = line.strip()
            if item.startswith("- "):
                if not isinstance(data.get(key), list):
                    data[key] = []
                data[key].append(item[2:].strip().strip('"').strip("'"))
            elif ":" in item:
                if not isinstance(data.get(key), dict):
                    data[key] = {}
                k, _, v = item.partition(":")
                data[key][k.strip()] = v.strip().strip('"').strip("'")
            continue
        km = re.match(r"^([a-z_]+):\s*(.*)$", line)
        if km:
            key, value = km.group(1), km.group(2).strip()
            # 行内列表形如 ports: [11091] 或 mac: ["aa:bb"]，按项拆开。
            inline = re.match(r"^\[(.*)\]$", value)
            if inline:
                items = [v.strip().strip('"').strip("'")
                         for v in inline.group(1).split(",") if v.strip()]
                data[key] = items
            else:
                data[key] = value.strip('"').strip("'") if value else None
    return data, m.group(2)


def gather_db(psql):
    tables = ["hardware_details", "machine_details", "software_instance_details",
              "product_details", "document_details", "runbook_details",
              "software_project_details", "account_credential_details",
              "network_details", "work_item_details"]
    union = " UNION ALL ".join(
        f"SELECT node_id, details::text AS d FROM {t}" for t in tables)
    sql = (f"SELECT n.id, n.name, coalesce(n.lifecycle,''), "
           f"coalesce((SELECT 1 FROM relations r WHERE r.to_id=n.id AND r.predicate='hosts' LIMIT 1),0), "
           f"coalesce(x.d,'') FROM nodes n LEFT JOIN ({union}) x ON x.node_id=n.id")
    out = subprocess.run(psql, shell=True, input=sql,
                         capture_output=True, text=True).stdout
    db = {}
    for line in out.split("\n"):
        if not line.strip():
            continue
        parts = line.split("|", 4)
        if len(parts) < 5:
            continue
        node_id, name, lifecycle, has_hosts, details = parts
        entry = db.setdefault(node_id, {"node": {"name": name, "lifecycle": lifecycle},
                                        "rel": {"hosts": has_hosts == "1"},
                                        "details": {}})
        if details:
            try:
                entry["details"] = json.loads(details)
            except json.JSONDecodeError:
                pass
    return db


def norm(value):
    """把值规整成可比较的形态。

    frontmatter 是文本，库里有布尔和整数，所以统一压成可比较的标量：布尔转
    小写字符串，数字转无小数点的字符串，列表逐项处理并排序。
    """
    if isinstance(value, list):
        return sorted(norm(v) for v in value)
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, (int, float)):
        return str(int(value)) if float(value).is_integer() else str(value)
    if isinstance(value, dict):
        return {k: norm(v) for k, v in value.items()}
    return str(value).strip()


def values_equal(hub_value, db_value):
    """判定 hub 值与库值是否表达同一件事。

    允许的差异：标量 vs 单元素列表、端口 int vs 字符串、大小写不敏感的
    true/false。其余差异都算不等，交由人工判断。
    """
    h, d = norm(hub_value), norm(db_value)
    if h == d:
        return True
    if isinstance(h, list) and not isinstance(d, list) and len(h) == 1:
        return h[0] == d
    if isinstance(d, list) and not isinstance(h, list) and len(d) == 1:
        return h == d[0]
    if isinstance(h, str) and isinstance(d, str):
        return h.lower() == d.lower()
    return False


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
    ap.add_argument("--values", action="store_true", help="显示值不等的明细")
    args = ap.parse_args()

    models = load_model_fields()
    mapping = load_mapping(args.config, args.wiki_root)
    db = gather_db(args.psql)

    compared = 0
    missing_field, value_diff, unmapped_key = [], [], []
    for rel, node_id in sorted(mapping.items()):
        fm, _body = read_frontmatter(os.path.join(args.wiki_root, rel))
        if not fm:
            continue
        source_type = (fm.get("type") or "").lower()
        model_name = TYPE_TO_MODEL.get(source_type)
        entry = db.get(node_id)
        if not entry:
            missing_field.append((rel, "(node)", "节点不在库中", None, None))
            continue
        if not model_name:
            continue
        allowed = models.get(model_name, set())

        for key, hub_value in fm.items():
            if key in META_KEYS or hub_value in (None, "", [], {}):
                continue
            # 1) 节点列
            if key in NODE_LEVEL:
                compared += 1
                db_value = entry["node"].get(NODE_LEVEL[key])
                if not values_equal(hub_value, db_value):
                    value_diff.append((rel, key, "node." + NODE_LEVEL[key], hub_value, db_value))
                continue
            # 2) 关系
            if key in RELATION_LEVEL:
                compared += 1
                if not entry["rel"].get(RELATION_LEVEL[key]):
                    missing_field.append((rel, key, "relation.hosts", hub_value, False))
                continue
            # 3) details 字段：不认识的键直接报出来，不静默跳过
            field = alias_for(key, model_name, allowed)
            if field not in allowed:
                unmapped_key.append((rel, key, model_name))
                continue
            compared += 1
            db_value = entry["details"].get(field)
            if db_value in (None, "", [], {}):
                missing_field.append((rel, key, f"{model_name}.{field}", hub_value, db_value))
            elif not values_equal(hub_value, db_value):
                value_diff.append((rel, key, f"{model_name}.{field}", hub_value, db_value))

    print(f"逐值比对 {compared} 个 frontmatter 事实")
    print(f"  字段缺失 : {len(missing_field)}")
    print(f"  值不一致 : {len(value_diff)}")
    print(f"  未覆盖键 : {len(unmapped_key)}")

    if unmapped_key:
        print("\n未覆盖键（frontmatter 有、模型和映射都没有 —— 说明事实没入库）：")
        for rel, key, model_name in unmapped_key:
            print(f"  {rel}: {key}  (模型 {model_name} 无此字段)")

    if missing_field:
        print("\n字段缺失：")
        for rel, key, target, hub_v, db_v in missing_field:
            print(f"  {rel}: {key} -> {target}  (hub={str(hub_v)[:60]!r})")

    if value_diff:
        print("\n值不一致：")
        for rel, key, target, hub_v, db_v in value_diff:
            if args.values:
                print(f"  {rel}: {key}")
                print(f"      hub = {hub_v!r}")
                print(f"      db  = {db_v!r}")
            else:
                print(f"  {rel}: {key} -> {target}")

    bad = len(missing_field) + len(value_diff) + len(unmapped_key)
    return 1 if (args.strict and bad) else 0


if __name__ == "__main__":
    sys.exit(main())
