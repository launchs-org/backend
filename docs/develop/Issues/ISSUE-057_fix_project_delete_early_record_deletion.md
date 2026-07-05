# ISSUE-057 プロジェクト削除時に terminating 中にレコードが消える問題の修正

## 親 Issue
なし（バグ修正）

## 概要
Project を削除すると、k8s Namespace が Terminating 状態にある段階で DB の Project レコードが削除されてしまう問題を修正する。

### k8s Watch の terminating / 完全削除の判定方法

k8s の Watch API では以下の挙動となる：

- `watch.Modified` イベント: `DeletionTimestamp != nil` → Terminating フェーズに入った（まだ存在する）
- `watch.Deleted` イベント: オブジェクトが etcd から完全に消えた → 完全削除済み

`watch.Deleted` は理論上「完全削除後」にのみ発火するが、現実の k8s クラスタでは Finalizer なし Namespace の場合に Terminating フェーズをスキップして即 `watch.Deleted` が来ることがある。

### 原因
`WatchNamespaces` の `handleNamespaceEvent`（`app/src/k8s/namespace.go:140`）は `watch.Deleted` イベントを受信した際に `project.status` が `deleting` かどうかを確認せずに即座に `projectRepo.DeleteNoTx` を呼び出している。

- API の `DeleteProject` が `project.status = deleting` を DB に書き込んだ後、goroutine で k8s Namespace 削除を実行する
- Namespace 削除 API を叩いた瞬間に `watch.Deleted` が発火し、DB 書き込み完了前にハンドラーが走ると status が `deleting` でない状態でレコードが消える
- また `deleting` 以外の状態（外部から Namespace が消された等）でも無条件にレコードを削除してしまう

正しい動作は「`project.status == deleting` の場合のみ DB レコードを削除する」であり、`deleting` 以外のステータスの場合は意図しない削除とみなして警告ログを出力して終了する。

## 変更ファイル一覧

- `app/src/k8s/namespace.go`（編集）
    - 何を: `handleNamespaceEvent` に `project.status == deleting` チェックを追加し、`deleting` でない場合はレコードを削除せずに警告ログを出力して終了する
    - なぜ: status チェックなしに削除するため terminating 中にレコードが消えるバグを修正するため

## テスト確認項目

- [ ] Project を DELETE した後、k8s Namespace が Terminating になっても UI 上の Project が消えないこと（削除中ステータスで表示されること）
- [ ] k8s Namespace が完全に削除された後、DB の Project レコードが削除されること
- [ ] Project の status が `active` など `deleting` 以外の場合に Namespace Deleted イベントが来ても DB レコードが削除されないこと（警告ログが出ること）
