#!/usr/bin/env python3
"""
GitHub Issue 同期スクリプト
docs/Issues/ 以下のマークダウンファイルを読み込み、
gh コマンドで GitHub Issue を作成または更新する。
既存 Issue はタイトル先頭の "ISSUE-XXX" で照合する（タイトル変更後も正しく更新される）。
Parent Issue には Sub Issue のリンクを追加し、
Sub Issue には親 Issue のリンクを追加する。
"""

import os
import re
import json
import subprocess
import sys
import time
from pathlib import Path
from dataclasses import dataclass, field


REPO = "launchs-org/backend"
ISSUES_DIR = Path(__file__).parent.parent / "docs" / "Issues"


@dataclass
class IssueData:
    file_path: Path
    issue_number: int          # ISSUE-XXX の番号
    title: str
    body: str
    parent_issue_number: int | None   # 親 Issue の番号（ISSUE-XXX）
    sub_issue_numbers: list[int]      # Sub Issue の番号リスト
    is_parent: bool
    github_number: int | None = None  # 作成後の GitHub Issue 番号


def run_gh(args: list[str], retries: int = 3, delay: float = 5.0) -> str:
    """gh コマンドを実行して stdout を返す。タイムアウト時はリトライする。"""
    last_error = None
    for attempt in range(1, retries + 1):
        result = subprocess.run(
            ["gh"] + args,
            capture_output=True,
            text=True,
        )
        if result.returncode == 0:
            return result.stdout.strip()
        last_error = subprocess.CalledProcessError(
            result.returncode,
            ["gh"] + args,
            output=result.stdout,
            stderr=result.stderr,
        )
        if "timeout" in result.stderr.lower() or "i/o timeout" in result.stderr.lower():
            print(f" [タイムアウト、{delay}秒後にリトライ {attempt}/{retries}]", end="", flush=True)
            time.sleep(delay)
            continue
        raise last_error
    raise last_error


def parse_issue_file(file_path: Path) -> IssueData:
    """マークダウンファイルを解析して IssueData を返す"""
    content = file_path.read_text(encoding="utf-8")

    # ファイル名から Issue 番号を取得 (ISSUE-002_db_setup.md -> 2)
    match = re.match(r"ISSUE-(\d+)", file_path.name)
    if not match:
        raise ValueError(f"ファイル名が不正です: {file_path.name}")
    issue_number = int(match.group(1))

    # タイトル取得（最初の # 行）
    title_match = re.search(r"^# (.+)$", content, re.MULTILINE)
    title = title_match.group(1).strip() if title_match else file_path.stem

    # 親 Issue 番号を取得
    parent_match = re.search(r"## 親 Issue\s+ISSUE-(\d+)", content)
    parent_issue_number = int(parent_match.group(1)) if parent_match else None

    # Sub Issues を取得
    sub_issue_numbers = []
    sub_issues_section = re.search(
        r"## Sub Issues\s+((?:- \[.\] ISSUE-\d+.*\n?)+)", content
    )
    if sub_issues_section:
        sub_lines = sub_issues_section.group(1)
        for sub_match in re.finditer(r"ISSUE-(\d+)", sub_lines):
            sub_issue_numbers.append(int(sub_match.group(1)))

    is_parent = len(sub_issue_numbers) > 0

    return IssueData(
        file_path=file_path,
        issue_number=issue_number,
        title=title,
        body=content,
        parent_issue_number=parent_issue_number,
        sub_issue_numbers=sub_issue_numbers,
        is_parent=is_parent,
    )


def load_all_issues() -> dict[int, IssueData]:
    """全 Issue ファイルを読み込んで辞書で返す"""
    issues: dict[int, IssueData] = {}
    for md_file in sorted(ISSUES_DIR.glob("ISSUE-*.md")):
        issue_data = parse_issue_file(md_file)
        issues[issue_data.issue_number] = issue_data
        print(f"  読み込み: ISSUE-{issue_data.issue_number:03d} {issue_data.title}")
    return issues


def get_existing_issues() -> dict[int, int]:
    """
    既存の GitHub Issue を取得する。
    issue_number (ISSUE-XXX の番号) -> github_number のマッピングを返す。
    タイトル先頭の "ISSUE-NNN" で照合するため、タイトルが変わっても正しく対応できる。
    """
    print("既存の GitHub Issue を取得中...")
    output = run_gh([
        "issue", "list",
        "--repo", REPO,
        "--state", "all",
        "--limit", "500",
        "--json", "number,title",
    ])
    issue_list = json.loads(output)
    result: dict[int, int] = {}
    for item in issue_list:
        match = re.match(r"ISSUE-(\d+)", item["title"])
        if match:
            result[int(match.group(1))] = item["number"]
    return result


def build_issue_body(issue_data: IssueData, issues: dict[int, IssueData]) -> str:
    """
    GitHub Issue 本文を構築する。
    - Sub Issue のリンクを末尾に追加する
    - 親 Issue のリンクを先頭に追加する
    """
    body = issue_data.body

    # 親 Issue リンクを先頭に追加する
    if issue_data.parent_issue_number is not None:
        parent = issues.get(issue_data.parent_issue_number)
        if parent and parent.github_number:
            parent_link = (
                f"> **親 Issue**: #{parent.github_number} "
                f"ISSUE-{parent.issue_number:03d} {parent.title}\n\n"
            )
            body = parent_link + body

    # Sub Issue リンクを末尾に追加する
    if issue_data.sub_issue_numbers:
        sub_lines = ["\n---\n## Sub Issues (GitHub リンク)\n"]
        for sub_num in issue_data.sub_issue_numbers:
            sub = issues.get(sub_num)
            if sub and sub.github_number:
                sub_lines.append(
                    f"- #{sub.github_number} ISSUE-{sub.issue_number:03d} {sub.title}"
                )
            else:
                sub_lines.append(f"- ISSUE-{sub_num:03d} (未作成)")
        body = body + "\n".join(sub_lines)

    return body


def create_issue(issue_data: IssueData, body: str) -> int:
    """gh コマンドで Issue を作成して GitHub Issue 番号を返す"""
    labels = ["parent-issue"] if issue_data.is_parent else []

    args = [
        "issue", "create",
        "--repo", REPO,
        "--title", issue_data.title,
        "--body", body,
    ]
    if labels:
        args += ["--label", ",".join(labels)]

    output = run_gh(args)
    # URL から番号を取得する (https://github.com/org/repo/issues/123)
    number_match = re.search(r"/issues/(\d+)$", output)
    if not number_match:
        raise ValueError(f"Issue 番号を取得できませんでした: {output}")
    return int(number_match.group(1))


def update_issue(github_number: int, new_title: str, new_body: str):
    """既存 Issue のタイトルと本文を更新する"""
    run_gh([
        "issue", "edit", str(github_number),
        "--repo", REPO,
        "--title", new_title,
        "--body", new_body,
    ])


def main():
    print(f"=== GitHub Issue 作成スクリプト ===")
    print(f"リポジトリ: {REPO}")
    print(f"Issues ディレクトリ: {ISSUES_DIR}\n")

    if not ISSUES_DIR.exists():
        print(f"エラー: {ISSUES_DIR} が存在しません")
        sys.exit(1)

    # 全 Issue を読み込む
    print("Issue ファイルを読み込み中...")
    issues = load_all_issues()
    print(f"合計 {len(issues)} 件の Issue を読み込みました\n")

    # 既存の GitHub Issue を取得してスキップ判定に使う
    existing = get_existing_issues()

    # ===== Phase 1: Sub Issue を作成・更新する =====
    # 親リンクは Phase 3 で追記するため、ここでは本文のみ更新する
    print("\n--- Phase 1: Sub Issue を作成・更新 ---")
    for issue_num in sorted(issues.keys()):
        issue_data = issues[issue_num]
        if issue_data.is_parent:
            continue  # parent は後で作成する

        if issue_data.issue_number in existing:
            issue_data.github_number = existing[issue_data.issue_number]
            print(
                f"  本文更新 (既存 #{issue_data.github_number}): {issue_data.title} ...",
                end="",
                flush=True,
            )
            update_issue(issue_data.github_number, issue_data.title, issue_data.body)
            print(" 完了")
            continue

        print(f"  作成中: ISSUE-{issue_num:03d} {issue_data.title} ...", end="", flush=True)
        github_number = create_issue(issue_data, issue_data.body)
        issue_data.github_number = github_number
        print(f" -> #{github_number}")

    # ===== Phase 2: Parent Issue を作成する =====
    print("\n--- Phase 2: Parent Issue を作成 ---")
    for issue_num in sorted(issues.keys()):
        issue_data = issues[issue_num]
        if not issue_data.is_parent:
            continue

        body_with_links = build_issue_body(issue_data, issues)

        if issue_data.issue_number in existing:
            issue_data.github_number = existing[issue_data.issue_number]
            print(
                f"  本文更新 (既存 #{issue_data.github_number}): {issue_data.title}"
            )
            update_issue(issue_data.github_number, issue_data.title, body_with_links)
            continue

        print(
            f"  作成中: ISSUE-{issue_num:03d} {issue_data.title} ...", end="", flush=True
        )
        github_number = create_issue(issue_data, body_with_links)
        issue_data.github_number = github_number
        print(f" -> #{github_number}")

    # ===== Phase 3: Sub Issue の本文に親リンクを追記する =====
    print("\n--- Phase 3: Sub Issue に親リンクを追記 ---")
    for issue_num in sorted(issues.keys()):
        issue_data = issues[issue_num]
        if issue_data.is_parent or issue_data.parent_issue_number is None:
            continue
        if issue_data.github_number is None:
            continue

        parent = issues.get(issue_data.parent_issue_number)
        if parent and parent.github_number:
            print(
                f"  更新中: ISSUE-{issue_num:03d} #{issue_data.github_number} "
                f"-> 親 #{parent.github_number}"
            )
            new_body = build_issue_body(issue_data, issues)
            update_issue(issue_data.github_number, issue_data.title, new_body)

    # ===== 完了サマリー =====
    print("\n=== 完了 ===")
    created = [i for i in issues.values() if i.github_number]
    print(f"処理した Issue: {len(created)} 件")
    for issue_data in sorted(created, key=lambda x: x.issue_number):
        print(f"  ISSUE-{issue_data.issue_number:03d} -> #{issue_data.github_number} {issue_data.title}")


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError as error:
        print(f"\n[ERROR] gh コマンド失敗: {' '.join(error.cmd)}")
        print(f"  stderr: {error.stderr}")
        sys.exit(1)
