#!/usr/bin/env python3
"""逐文件判定 hub 正文里剩余事实的去向，产出可复核清单。

结构化字段已由 parity-report.py 逐值校验。剩下的是正文：排错步骤、历史沿革、
决策理由、操作命令。这些要么入库（需要模型支持），要么明确保留为叙述。

判定规则：
  · 已入库重复  —— 正文里出现与 frontmatter 相同的字段值，可删
  · 叙述保留    —— 过程性知识，库无法表达，保留
  · 待补模型    —— 事实清晰但模型没有字段，需要先扩模型

输出 Markdown 表格，供人工逐行确认。
"""
import os
import re
import sys

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
WIKI = os.environ.get(
    "HUB_WIKI",
    "/home/dev/git-repo/codeup.aliyun.com/5ed719afd1d1abe63b55e7da/hub/hall-wiki")
CONFIG = os.environ.get("WIKI_SOURCE", "/home/dev/deploy/my-known-world/wiki-source.yaml")

# 正文里这些模式代表过程性/叙述性知识。
NARRATIVE_PATTERNS = {
    "排错": r"(排错|故障|报错|踩坑|不要|禁止|注意：)",
    "历史": r"(20\d\d-\d\d-\d\d|已停用|已退役|迁入|迁到|曾)",
    "决策": r"(因为|原因是|所以|权衡)",
    "操作": r"(```|执行|命令)",
}


def load_pairs():
    src = open(CONFIG, encoding="utf-8").read()
    return re.findall(r"sourceId:\s*(\S+)\s*\n\s*nodeId:\s*([0-9a-f-]{36})", src)


def read(path):
    text = open(path, encoding="utf-8").read()
    m = re.match(r"^---\n(.*?)\n---\n(.*)$", text, re.S)
    if not m:
        return {}, text
    fm, body = {}, m.group(1)
    key = None
    for line in m.group(1).split("\n"):
        if re.match(r"^\s+", line) and key:
            item = line.strip()
            if item.startswith("- "):
                fm.setdefault(key, [])
                if isinstance(fm[key], list):
                    fm[key].append(item[2:].strip().strip('"'))
            continue
        km = re.match(r"^([a-z_]+):\s*(.*)$", line)
        if km:
            key, value = km.group(1), km.group(2).strip()
            fm[key] = value.strip('"') if value else None
    return fm, m.group(2)


def dedupe_candidates(fm, body):
    """找出正文里重复了 frontmatter 值的地方。"""
    hits = []
    for key, value in fm.items():
        if key in ("type", "id", "name", "title") or not isinstance(value, str):
            continue
        if len(value) < 6:
            continue
        if value in body:
            hits.append(key)
    return hits


def main():
    pairs = load_pairs()
    rows = []
    for rel, uid in pairs:
        fm, body = read(os.path.join(WIKI, rel))
        tags = [name for name, pat in NARRATIVE_PATTERNS.items() if re.search(pat, body)]
        dup = dedupe_candidates(fm, body)
        declared = "以 my-known-world 为准" in body
        if declared:
            verdict = "已声明以库为准"
        elif dup:
            verdict = "正文含已入库字段值，可剥离"
        elif tags:
            verdict = "叙述保留"
        else:
            verdict = "需人工判断"
        lines = [l for l in body.split("\n") if l.strip() and not l.startswith("#")]
        rows.append({
            "file": rel, "uuid": uid, "lines": len(lines),
            "chars": sum(len(l) for l in lines),
            "tags": "/".join(tags) or "-",
            "dup": ",".join(dup) or "-",
            "verdict": verdict,
        })

    rows.sort(key=lambda r: -r["chars"])
    print("| 文件 | 正文行 | 字数 | 叙述特征 | 重复字段 | 判定 |")
    print("|---|---|---|---|---|---|")
    for r in rows:
        print(f"| {r['file']} | {r['lines']} | {r['chars']} | {r['tags']} | {r['dup']} | {r['verdict']} |")

    from collections import Counter
    c = Counter(r["verdict"] for r in rows)
    print(f"\n合计 {len(rows)} 个文件，正文 {sum(r['chars'] for r in rows)} 字")
    for k, v in c.most_common():
        print(f"  {v:3}  {k}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
