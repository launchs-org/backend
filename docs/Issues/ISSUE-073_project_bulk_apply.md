# ISSUE-073 プロジェクト単位の一括 apply 機能

## 親 Issue
なし（独立 Issue）

## 概要

現状 apply は Deployment 単位（`POST /deployments/:id/apply`）でしか実行できず、複数 Deployment に変更がある場合はユーザーが1件ずつ Apply ボタンを押す必要がある。
また `POST /projects/:id/apply` は既に存在するが、これは IngressRoute（PathRule 含む）専用であり、Deployment 本体の pending（イメージ・環境変数・ボリューム・レプリカ数など）は対象外。

本 Issue では `POST /projects/:id/apply` を拡張し、プロジェクト配下の「全 Deployment の pending」と「IngressRoute/PathRule の pending」を一括で apply できるようにする。
フロントエンドはプロジェクト画面に一括 Apply ボタンを新設し、クリック時に画面中央の確認ダイアログを表示、確認後に一括 apply を実行する。

既存の Deployment 単位 Apply（個別ボタン・エンドポイント）は変更せず、そのまま残す。

## 実装方針

### バックエンド

新規の Temporal Workflow は作らない。既存の `ApplyService.Apply(ctx, userID, deploymentID)` を、プロジェクト配下で pending がある Deployment 全件に対して順次呼び出し、その後に既存の IngressRoute apply ロジック（`applySingleIngressRoute` ループ）を実行する形で `ApplyProject` を拡張する。

実行順序: **Deployment 群 → IngressRoute**（Pod が新しい状態になってからルーティングを切り替える）。

エラー処理: 一部の Deployment が失敗（クオータ超過、ステータス不正など）してもその Deployment はスキップして処理を継続し、他の Deployment・IngressRoute の apply は実行する。最終的に成功/失敗件数と失敗理由をレスポンスで返す。

#### 対象ファイル

- `handler/src/service/apply.go`（編集）
    - 何を:
        - `ApplyProjectResult` 構造体を新設する（例: `AppliedDeploymentCount int`, `FailedDeploymentList []ApplyProjectFailure`, `IngressRouteApplied bool`）
        - `ApplyProjectFailure` 構造体を新設する（`DeploymentID string`, `Error string`）
        - `ApplyProject` の戻り値を `error` から `(*ApplyProjectResult, error)` に変更する
        - `ApplyProject` 内で `DeploymentRepo.FindAllByProjectID` を呼び、pending の有無を判定するヘルパー（例: `hasPendingChanges(deploymentData *models.Deployment) bool`）で対象を絞り込む
        - pending がある Deployment 各件に対して既存の `applyService.Apply(ctx, userID, deploymentData.ID)` をループ呼び出しする。エラーはスキップして `FailedDeploymentList` に積む
        - Deployment 群の apply が終わった後、既存の IngressRoute apply ループ（`FindAllByProjectID` → `applySingleIngressRoute`）を実行する
    - なぜ: Deployment 単位の既存バリデーション（所有権・ステータス・クオータチェック）と Temporal Workflow 起動ロジックをそのまま再利用し、実装コストとリスクを抑えるため

- `handler/src/handler/ingress_route_handler.go`（編集）
    - 何を: `ApplyProject` ハンドラーが新しい戻り値 `*ApplyProjectResult` を JSON レスポンスとして返すようにする（`AppliedDeploymentCount`, `FailedDeploymentList`, `IngressRouteApplied` を含む）
    - なぜ: フロントエンドが一括 apply の結果件数を表示できるようにするため

> **pending 判定の条件**（`hasPendingChanges`）: Deployment の各 `pending_*` フィールド（`PendingImageID`, `PendingGithubRepoURL`, `PendingGithubBranch`, `PendingGithubCommitSHA`, `PendingGithubRepoDirectory`, `PendingDockerfilePath`, `PendingInstanceSize`, `PendingReplicas`, `PendingCommand`, `PendingArgs`）のいずれかが現在値と異なる場合に pending ありと判定する。加えて Service/EnvVarMount/VolumeMount に `status=pending` または `status=deleting` のレコードがある場合も pending ありとみなす（フロントの `DeploymentDetailPage.tsx` の `fetchAllPending` と同じ考え方）。ただしこれらの判定は `Apply(ctx, userID, deploymentID)` 自体の中で改めて評価されるため、`ApplyProject` 側では「apply 対象に含めるかどうか」の粗い判定（`pending_replicas` 等の主要フィールドの差分チェック）で十分。誤って pending なし Deployment を含めても `Apply` 内部の処理自体は副作用がない点に注意する。

### フロントエンド

プロジェクト画面配下の全 Deployment の pending 有無を、Deployment ごとに複数 API（service / env-var-mounts / volume-mounts 等）を呼んで集計すると `Deployment数 × 5 API` のコストがかかり非効率なため、**バックエンドに集計用エンドポイントを追加する**。

#### 対象ファイル

- `handler/src/handler/project_handler.go` または `deployment_handler.go`（編集、または新規）
    - 何を: `GET /projects/:id/pending-summary` を新設し、`{ "has_pending": bool, "pending_deployment_count": number, "pending_ingress_route_count": number }` 相当を返す
    - なぜ: プロジェクト画面で一括 Apply ボタンの表示可否・確認ダイアログの件数表示に必要な集計をフロント側の多重 API 呼び出しなしで取得するため

- `frontend/src/pages/ProjectDetailPage.tsx`（編集）
    - 何を:
        - 上記の `pending-summary` を定期ポーリング（既存の10秒ポーリングと合わせる）で取得し `hasPendingSummary` state を保持する
        - `hasPendingSummary.has_pending` が true の場合のみ画面上部に「一括 Apply」ボタンを表示する
        - ボタンクリックで `confirm-dialog.tsx` の `ConfirmDialog` を開き、pending 件数（Deployment N件 / IngressRoute M件）を確認メッセージに表示する
        - OK 押下で `post(\`/projects/${projectId}/apply\`)` を実行し、成功後に `fetchData` を再実行して画面を更新する。レスポンスの `FailedDeploymentList` が1件以上ある場合は失敗件数を toast 等で警告表示する
    - なぜ: 「プロジェクト単位の apply をデフォルトにしたい」という要望に対し、既存の削除確認ダイアログと同じパターン（`ConfirmDialog` 再利用）で一貫した UI を提供するため

既存の `IngressRouteSidebar` 内の個別 Apply 導線（1049行目付近）はそのまま残す。今回追加するのはプロジェクト画面のメイン領域に出す一括 Apply ボタン・ダイアログであり、別の導線として共存させる。

## テスト確認項目

- [ ] プロジェクト配下に pending な Deployment が複数ある状態で `POST /projects/:id/apply` を呼ぶと、全 Deployment の apply が呼ばれ、IngressRoute も反映されること
- [ ] 一部の Deployment がクオータ超過などで失敗しても、他の Deployment・IngressRoute の apply は継続されること
- [ ] レスポンスに成功/失敗件数が正しく含まれること
- [ ] pending がない Deployment は apply 対象に含まれない（無駄な Workflow が起動されない）こと
- [ ] 所有者でない `userID` から呼び出した場合 `ErrForbidden` が返ること
- [ ] `GET /projects/:id/pending-summary` が pending の有無・件数を正しく返すこと
- [ ] フロント: pending が0件の場合、一括 Apply ボタンが表示されないこと
- [ ] フロント: 一括 Apply ボタンクリックで画面中央に確認ダイアログが表示されること
- [ ] フロント: ダイアログで OK を押すと一括 apply が実行され、画面が更新されること
- [ ] フロント: 一部失敗時に警告が表示されること
