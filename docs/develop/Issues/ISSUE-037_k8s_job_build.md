# ISSUE-037 k8s Job（ビルド）操作

## 親 Issue
ISSUE-035

## 概要
Dockerfileまたはrailpackを使ったコンテナビルドをk8s Jobとして実行する操作関数を実装する。Harborへのpushも含める。

## 変更ファイル一覧

- `app/src/k8s/build.go`（新規作成）
    - **何を**: CreateBuildJob関数の実装。ビルドタイプ（dockerfile/railpack）に応じてk8s Jobを生成する。GitHubリポジトリのcloneとHarborへのdocker build + pushを実行するJobコンテナの定義。HarborのロボットアカウントのシークレットをJobのenv変数に渡す。DeleteBuildJob関数の実装。
    - **なぜ**: コンテナビルド処理をk8s Jobとして委譲するため

## テスト確認項目

- [ ] dockerfile指定のJobが正常に作成されること
- [ ] railpack指定のJobが正常に作成されること
- [ ] Harbor認証情報がJobのenv変数に正しく設定されること
- [ ] k8s Jobが正常に削除されること
