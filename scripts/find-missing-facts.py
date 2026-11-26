#!/usr/bin/env python3
"""列出每个服务文件正文里尚未入库的事实，作为补字段的依据。

剥离正文前必须确认真值已经在库里，否则删了就没了。本脚本把正文里的
「键：值」形式的事实提取出来，与 frontmatter 现有键对比，指出缺口。
"""
import os
import re
import sys

WIKI = os.environ.get(
    "HUB_WIKI",
    "/home/dev/git-repo/codeup.aliyun.com/5ed719afd1d1abe63b55e7da/hub/hall-wiki")
CONFIG = os.environ.get("WIKI_SOURCE", "/home/dev/deploy/my-known-world/wiki-source.yaml")

# 正文里「X：值」形态的字段，能对上模型字段的就归为事实。
FACT_HINTS = [
    ("deploy_path", r"部署目录[：:]\s*`([^`]+)`"),
    ("container", r"容器\s*`([^`]+)`|容器[：:]\s*`([^`]+)`"),
    ("ports", r"宿主\s*`?(\d+)`?\s*→\s*容器"),
    ("domain", r"入口[：:]\s*`?(https?://[^`\s]+)"),
    ("notes", r"restart policy\s*`?([a-zA-Z-]+)`?"),
    ("notes", r"数据[：:]\s*`([^`]+)`"),
]


def read(path):
    text = open(path, encoding="utf-8").read()
    m = re.match(r"^---\n(.*?)\n---\n(.*)$", text, re.S)
    if not m:
        return set(), text
    keys = set()
    for line in m.group(1).split("\n"):
        km = re.match(r"^([a-z_]+):", line)
        if km:
            keys.add(km.group(1))
    return keys, m.group(2)


def main():
    src = open(CONFIG, encoding="utf-8").read()
    pairs = re.findall(r"sourceId:\s*(\S+)\s*\n\s*nodeId:\s*([0-9a-f-]{36})", src)
    for rel, uid in pairs:
        keys, body = read(os.path.join(WIKI, rel))
        found = []
        for field, pat in FACT_HINTS:
            for m in re.finditer(pat, body):
                value = next((g for g in m.groups() if g), "")
                if value:
                    found.append((field, value))
        if not found:
            continue
        gaps = [(f, v) for f, v in found if f not in keys]
        if not gaps:
            continue
        print(f"{rel}")
        for field, value in gaps:
            print(f"    缺 {field:12} <- {value[:70]}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
