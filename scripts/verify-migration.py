#!/usr/bin/env python3
"""hub 原文 → my-known-world 迁移的数据完整性校验。

原则：**原始数据不丢失**。任何一批迁移前后，都必须能证明 hub 原文里
出现过的每条事实，要么仍在 hub 原文里，要么已能在库中查到。

用法：
  verify-migration.py snapshot   # 迁移前，对指定文件生成事实基线
  verify-migration.py compare    # 迁移后，逐条核对基线中的事实是否仍有归属

基线文件：~/backups/hub/migration-baseline.json
"""
import json
import os
import re
import sys

# Override to point at another checkout: HUB_WIKI=/path/to/hub/hall-wiki
HUB_WIKI = os.environ.get(
    "HUB_WIKI",
    "/home/dev/git-repo/codeup.aliyun.com/5ed719afd1d1abe63b55e7da/hub/hall-wiki",
)
BASELINE = os.path.expanduser("~/backups/hub/migration-baseline.json")

# 值得保护的事实形态。宁可多抓，不可漏抓——漏抓会让删除变得安全得假象。
PATTERNS = [
    ("ip", re.compile(r"\b\d{1,3}(?:\.\d{1,3}){3}\b")),
    ("path", re.compile(r"`?(/[A-Za-z0-9._/@-]+(?:/[A-Za-z0-9._@-]+)+)`?")),
    ("container", re.compile(r"\b(?:hall-[a-z0-9][a-z0-9-]*|[a-z0-9-]+-[a-z0-9-]+-\d+)\b")),
    ("port", re.compile(r"\b\d{2,5}/(?:tcp|udp)\b")),
    ("hostname", re.compile(r"\b[a-z0-9][a-z0-9-]*\.(?:hp\.hall|svc\.hp\.hall|hall)\b")),
    ("version", re.compile(r"\b(?:v|experimental-)[0-9][A-Za-z0-9._-]*\b")),
    ("date", re.compile(r"\b20\d{2}-\d{2}-\d{2}\b")),
]


def split_frontmatter(text):
    if not text.startswith("---"):
        return "", text
    parts = text.split("---", 2)
    if len(parts) < 3:
        return "", text
    return parts[1], parts[2]


# Values that must never be kept in hub text; they live in the password store.
# Excluding them keeps the verifier honest about what "not lost" means.
SECRET_LITERALS = set()


def collect_facts(text):
    """Return {(kind, value)} for every recognisable fact in text."""
    facts = set()
    for kind, pattern in PATTERNS:
        for match in pattern.findall(text):
            value = match if isinstance(match, str) else match[0]
            if kind == "path" and value.count("/") < 2:
                continue
            facts.add((kind, value))
    return {f for f in facts if f[1] not in SECRET_LITERALS}


def iter_docs(root):
    for dirpath, _dirnames, filenames in os.walk(root):
        for name in sorted(filenames):
            if not name.endswith(".md"):
                continue
            path = os.path.join(dirpath, name)
            rel = os.path.relpath(path, root)
            yield rel, path


def snapshot():
    data = {}
    for rel, path in iter_docs(HUB_WIKI):
        text = open(path, encoding="utf-8").read()
        _front, body = split_frontmatter(text)
        facts = collect_facts(text)
        data[rel] = {
            "body_facts": sorted(f"{k}:{v}" for k, v in collect_facts(body)),
            "all_facts": sorted(f"{k}:{v}" for k, v in facts),
        }
    os.makedirs(os.path.dirname(BASELINE), exist_ok=True)
    with open(BASELINE, "w", encoding="utf-8") as fh:
        json.dump(data, fh, ensure_ascii=False, indent=1, sort_keys=True)
    os.chmod(BASELINE, 0o600)
    total = sum(len(v["all_facts"]) for v in data.values())
    print(f"基线已写入 {BASELINE}")
    print(f"文件 {len(data)} 个，事实 {total} 条")


def compare():
    if not os.path.exists(BASELINE):
        sys.exit("先运行 snapshot 建立基线")
    baseline = json.load(open(BASELINE, encoding="utf-8"))
    lost = {}
    for rel, old in baseline.items():
        path = os.path.join(HUB_WIKI, rel)
        if not os.path.exists(path):
            lost[rel] = ["<整个文件已删除>"] + old["all_facts"]
            continue
        text = open(path, encoding="utf-8").read()
        current = {v for v in collect_facts(text)}
        current_flat = {f"{k}:{v}" for k, v in current}
        missing = [f for f in old["all_facts"] if f not in current_flat]
        if missing:
            lost[rel] = missing

    if not lost:
        print("校验通过：基线中的全部事实在当前 hub 原文中仍可找到")
        return 0

    print("以下事实已从 hub 原文消失，必须确认它们已在库中可查：")
    total = 0
    for rel, facts in sorted(lost.items()):
        print(f"\n  {rel}")
        for fact in facts:
            print(f"    - {fact}")
        total += len(facts)
    print(f"\n共 {total} 条待核对")
    return 1


if __name__ == "__main__":
    if len(sys.argv) < 2 or sys.argv[1] not in ("snapshot", "compare"):
        sys.exit(__doc__)
    sys.exit(snapshot() if sys.argv[1] == "snapshot" else compare())
