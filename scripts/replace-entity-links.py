#!/usr/bin/env python3
"""把 hall-wiki 里指向实体的 Markdown 链接改写成节点 UUID。

my-known-world 是权威数据源，hub 应是投影层：引用实体时给出库里的节点 UUID，
而不是文件路径。文件可以改名、移动，UUID 不会变。

范围：
  * 只处理指向「有 frontmatter 的实体文件」的链接，其余链接（AGENTS.md、
    code/ 下的文档、其他仓库）保持原样。
  * README 这类索引文件跳过——它是给人导航用的表格，链接本身是价值，
    改由调用方另行补 UUID 列。

用法：
  python3 scripts/replace-entity-links.py --dry-run   # 只报告
  python3 scripts/replace-entity-links.py --apply     # 实际改写
"""
import argparse
import os
import re
import sys


def load_mapping(config_path, wiki_root):
    """从 wiki-source.yaml 读出 sourceId -> nodeId，并只保留实际存在实体文件。"""
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
                path = os.path.join(wiki_root, source_id)
                if os.path.exists(path):
                    mapping[source_id] = m.group(1)
                source_id = None
    return mapping


def is_index_file(rel_path):
    """导航索引文件，保留链接原样。

    注意：实体目录也常用 README.md 作为档案（如 svc/gitea/README.md），
    那些是有 frontmatter 的实体文件，必须参与改写。真正的索引是仓库根与
    分类目录下的 README。
    """
    if os.path.basename(rel_path).lower() != "readme.md":
        return False
    return rel_path.count(os.sep) <= 1


LINK_RE = re.compile(r"\[([^\]]+)\]\(([^)#\s]+\.md)(#[^)]*)?\)")


def rewrite_file(path, rel_from_root, mapping, wiki_root, apply):
    """返回 (改动数, 新内容)。"""
    with open(path, encoding="utf-8") as fh:
        original = fh.read()

    changes = []

    def replace(match):
        text, target, anchor = match.group(1), match.group(2), match.group(3) or ""
        resolved = os.path.normpath(
            os.path.join(os.path.dirname(rel_from_root), target)
        )
        uuid = mapping.get(resolved)
        if not uuid:
            return match.group(0)
        # 保留原显示文字，把目标换成 UUID。
        changes.append((target, uuid))
        return f"[{text}]({uuid}{anchor})"

    updated = LINK_RE.sub(replace, original)
    if apply and updated != original:
        with open(path, "w", encoding="utf-8") as fh:
            fh.write(updated)
    return len(changes), updated


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
    if not mapping:
        print("错误：没有读到任何 实体文件 -> UUID 映射", file=sys.stderr)
        return 1
    print(f"实体映射: {len(mapping)} 条")

    total = 0
    touched = 0
    skipped_index = 0
    for root, _dirs, names in os.walk(args.wiki_root):
        for name in sorted(names):
            if not name.endswith(".md"):
                continue
            full = os.path.join(root, name)
            rel = os.path.relpath(full, args.wiki_root)
            if is_index_file(rel):
                skipped_index += 1
                continue
            count, _ = rewrite_file(full, rel, mapping, args.wiki_root, args.apply)
            if count:
                touched += 1
                total += count
                print(f"  {count:3}  {rel}")

    verb = "已改写" if args.apply else "将改写"
    print(f"\n{verb} {total} 处链接，涉及 {touched} 个文件；跳过索引文件 {skipped_index} 个")
    if not args.apply:
        print("（未写入，加 --apply 执行）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
