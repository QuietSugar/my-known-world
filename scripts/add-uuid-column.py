#!/usr/bin/env python3
"""给索引类 README 的表格补一列节点 UUID。

索引表的作用是给人导航，文件链接本身有价值，所以不替换链接；但表里同时
给出该行对应的节点 UUID，读者（或 agent）可以直接拿去查库。

做法：逐行扫描 Markdown 表格；若某行第一个指向「实体文件」的链接能匹配到
UUID，就在该行末尾追加一列。表头与分隔行同步补齐。无匹配的行留空。

用法：
  python3 scripts/add-uuid-column.py --dry-run
  python3 scripts/add-uuid-column.py --apply
"""
import argparse
import os
import re
import sys


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


LINK_RE = re.compile(r"\[[^\]]+\]\(([^)#\s]+\.md)(?:#[^)]*)?\)")


def find_row_uuid(row, rel_from_root, mapping):
    for target in LINK_RE.findall(row):
        resolved = os.path.normpath(
            os.path.join(os.path.dirname(rel_from_root), target)
        )
        if resolved in mapping:
            return mapping[resolved]
    return ""


def process(path, rel_from_root, mapping, apply):
    with open(path, encoding="utf-8") as fh:
        lines = fh.read().split("\n")

    out = []
    i = 0
    added = 0
    while i < len(lines):
        line = lines[i]
        # 表头行后紧跟分隔行才算表格
        if line.startswith("|") and i + 1 < len(lines) and re.match(r"^\|[\s:|-]+\|$", lines[i + 1].strip()):
            header = line
            sep = lines[i + 1]
            # 已有 uuid 列就跳过
            if "uuid" in header.lower():
                out.append(line)
                i += 1
                continue
            ncols = header.count("|") - 1
            out.append(header.rstrip() + " UUID |")
            out.append(sep.rstrip() + "---|")
            i += 2
            while i < len(lines) and lines[i].startswith("|"):
                row = lines[i]
                uuid = find_row_uuid(row, rel_from_root, mapping)
                if uuid:
                    added += 1
                base = row.rstrip()
                # 补齐缺的单元格，保证新列落在正确位置
                have = base.rstrip("|").count("|")
                if have < ncols:
                    base = base.rstrip("|") + "|" + "|".join([""] * (ncols - have))
                out.append(base.rstrip() + f" `{uuid}` |")
                i += 1
            continue
        out.append(line)
        i += 1

    if apply:
        with open(path, "w", encoding="utf-8") as fh:
            fh.write("\n".join(out))
    return added


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--wiki-root", default=os.environ.get(
        "HUB_WIKI",
        "/home/dev/git-repo/codeup.aliyun.com/5ed719afd1d1abe63b55e7da/hub/hall-wiki"))
    ap.add_argument("--config", default=os.environ.get(
        "WIKI_SOURCE", "/home/dev/deploy/my-known-world/wiki-source.yaml"))
    ap.add_argument("--apply", action="store_true")
    args = ap.parse_args()

    mapping = load_mapping(args.config, args.wiki_root)
    total = 0
    for root, _dirs, names in os.walk(args.wiki_root):
        for name in sorted(names):
            if not name.lower().endswith("readme.md"):
                continue
            full = os.path.join(root, name)
            rel = os.path.relpath(full, args.wiki_root)
            n = process(full, rel, mapping, args.apply)
            if n:
                total += n
                print(f"  {n:3}  {rel}")
    verb = "已补" if args.apply else "将补"
    print(f"\n{verb} {total} 行 UUID")
    return 0


if __name__ == "__main__":
    sys.exit(main())
