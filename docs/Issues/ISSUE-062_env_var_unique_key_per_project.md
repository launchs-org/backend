# ISSUE-062 シークレット名（環境変数キー）の重複禁止

## 親 Issue
なし

## 概要
同一プロジェクト内で同じ `key` を持つ環境変数（EnvVar）を作成・更新できないようにバリデーションを追加する。

### 背景
デプロイメントのマウント設定は `env_var_id`（UUID）で紐付けているため、同名キーが複数あっても k8s への apply は一応動作する。
ただし `EnvVarSidebar` のUI上で表示されるのはキー名だけであり、同名のシークレット・通常変数が並んでいると「どちらをマウントしているのか」視覚的に区別できない問題がある。
また k8s ConfigMap/Secret はキー名をマップのキーとして使用するため、同名の変数が複数マウントされた場合に後者で上書きされる副作用もある。

現在は DB のユニーク制約も、サービス層のチェックもないため、同名の環境変数が複数登録できてしまう。

### テンプレート適用時のサフィックス方式

テンプレートから環境変数を一括追加する際、キー名が既存の変数と衝突する場合がある（同じテンプレートを2回適用・別テンプレートでキーが被る等）。
重複禁止バリデーションを入れると `handleApplyEnvTemplate` の `for` ループが途中で 409 を受けて止まり、一部の変数だけ作成された中途半端な状態になる。

対策として **サフィックス方式** を採用する：

- `POST /projects/:id/env-vars` が 409 を返した場合、フロントエンドは `DATABASE_URL_2`、`DATABASE_URL_3` のように連番サフィックスを付けてリトライする
- リトライは最大 10 回まで試み、それでも衝突する場合はその変数のみスキップしてエラーログを出力する
- ユーザーは適用後に自分で正しいキー名にリネームできる（意図が伝わるキー名が残る）

```typescript
// フロントエンドの実装イメージ
async function createEnvVarWithSuffix(projectId: string, baseKey: string, value: string, isSecret: boolean): Promise<EnvVar | null> {
  for (let attempt = 0; attempt <= 10; attempt++) {
    const key = attempt === 0 ? baseKey : `${baseKey}_${attempt + 1}` // 衝突時はサフィックスを付ける
    try {
      return await post<EnvVar>(`/projects/${projectId}/env-vars`, { key, value, is_secret: isSecret })
    } catch (err) {
      if (!isConflict(err)) throw err // 409 以外は再スローする
    }
  }
  return null // 10回試みても衝突する場合はスキップする
}
```

## 変更ファイル一覧

- `app/src/models/env_var.go`（編集）
    - 何を: `Key` フィールドの GORM タグに `uniqueIndex:idx_env_var_project_key` を追加し、`ProjectID` との複合ユニーク制約を付与する
    - なぜ: DB レベルで同一プロジェクト内のキー重複を防止する最終防衛線を設けるため

- `app/src/repository/env_var_repository.go`（編集）
    - 何を: `ExistsByProjectIDAndKey(ctx context.Context, projectID string, key string, excludeID string) (bool, error)` メソッドをインターフェースと実装に追加する（`excludeID` は更新時に自分自身を除外するために使用する）
    - なぜ: サービス層でキー重複チェックを行うために必要なリポジトリメソッドを提供するため

- `app/src/service/env_var_service.go`（編集）
    - 何を: `CreateEnvVar` の冒頭と `UpdateEnvVar` の `Key` 更新前に `envVarRepo.ExistsByProjectIDAndKey` を呼び出し、重複がある場合は `errors.New("同じキーの環境変数がすでに存在します")` を返すバリデーションを追加する
    - なぜ: ハンドラー層に到達する前にビジネスロジック層で重複を検出し、409 Conflict として返すため

- `app/src/handler/env_var_handler.go`（編集）
    - 何を: `CreateEnvVar` および `UpdateEnvVar` ハンドラーで `service.ErrDuplicateEnvVarKey`（または `errors.Is` による判定）を検知し、`http.StatusConflict` (409) を返すエラーハンドリングを追加する
    - なぜ: API クライアントが重複エラーを区別できる HTTP ステータスコードを返すため

- `frontend/src/pages/DeploymentDetailPage.tsx`（編集）
    - 何を: `handleApplyEnvTemplate` 内の環境変数作成処理を `createEnvVarWithSuffix` ヘルパーに置き換え、409 が返った場合に `KEY_2`、`KEY_3` と連番サフィックスを付けてリトライするロジックを追加する
    - なぜ: テンプレート適用時のキー衝突で途中失敗する問題を防ぐため

## テスト確認項目

- [ ] 同一プロジェクトに同じ `key` の環境変数を 2 つ作成しようとすると 409 が返ること
- [ ] 別のプロジェクトでは同じ `key` を持つ環境変数を作成できること
- [ ] 環境変数の `key` を更新して既存の別の `key` と重複した場合に 409 が返ること
- [ ] 環境変数の `key` を同じ値のまま更新（変更なし）した場合は 200 が返ること（自己重複除外ロジックの確認）

### テンプレート適用テスト

- [ ] 同名キーを持つテンプレートを2回適用すると、2回目は `KEY_2` として作成されること
- [ ] 別テンプレートで同名キーが被った場合もサフィックスが付いて作成されること
- [ ] サフィックスを付けて作成された変数が自動的にマウントされること

### repository 層テスト

- [ ] `ExistsByProjectIDAndKey` が同一プロジェクト・同一キーで `true` を返すこと
- [ ] `ExistsByProjectIDAndKey` が `excludeID` に自分自身の ID を渡すと `false` を返すこと
