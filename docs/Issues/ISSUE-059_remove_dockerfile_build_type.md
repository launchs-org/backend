# ISSUE-059 dockerfile ビルドタイプの UI・バックエンドからの除去

## 親 Issue
なし

## 概要
Deployment 作成・更新フォームから `dockerfile` タイプを選択肢として除去する。
バックエンドも `dockerfile` タイプのビルドリクエストを受け付けないようにバリデーションを追加する。

### 背景
`dockerfile` タイプのビルドは k8s Job による実装（ISSUE-051）が未完了であり、builder イメージの設計も未確定のため現時点では利用不可。
フロントエンドに選択肢が表示されているにもかかわらず実際にはビルドが起動しないため、ユーザー体験を損なう。
実装が完了するまでの間、UI とバックエンドの双方から `dockerfile` タイプを非表示・拒否する。

## 変更ファイル一覧

- `frontend/src/pages/DeploymentNewPage.tsx`（編集）
    - 何を: ビルドタイプ選択肢の配列から `dockerfile` エントリを削除し、`type` の初期値を `railpack` または `image_url` に変更する
    - なぜ: ユーザーが未実装のタイプを選択できないようにするため

- `frontend/src/pages/DeploymentDetailPage.tsx`（編集）
    - 何を: タイプ表示ロジックの `dockerfile` ケースを削除し、関連する Dockerfile パスのフォームフィールドを非表示にする
    - なぜ: 既存の dockerfile タイプ Deployment がある場合でも UI 上で混乱させないため（将来の再有効化を前提に条件分岐を残す形でも可）

- `app/src/service/build_service.go`（編集）
    - 何を: `TriggerBuild` において `models.BuildTypeDockerfile` タイプが渡された場合に `errors.New("dockerfile タイプは現在サポートされていません")` を返すバリデーションを追加する
    - なぜ: フロントエンドをバイパスしたリクエストに対してもバックエンドで拒否するため

## テスト確認項目

- [ ] Deployment 作成フォームに `dockerfile` の選択肢が表示されないこと
- [ ] バックエンドに `type=dockerfile` でビルドリクエストを送ると 400 または適切なエラーが返ること
- [ ] `railpack` および `image_url` タイプの作成・ビルドフローが引き続き正常に動作すること
