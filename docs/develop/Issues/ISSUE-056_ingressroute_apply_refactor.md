# ISSUE-056 k8s IngressRoute apply ロジック変更（PathRule 集約）

## 親 Issue
ISSUE-052

## 概要

apply 時の IngressRoute 反映ロジックを、PathRule テーブルを集約して Traefik IngressRoute を再構築する形に変更する。
`status=active` と `status=pending` の PathRule を全件取得し、1つの Traefik IngressRoute に複数ルートとして書き込む。
apply 完了後、pending → active 昇格 / deleting → 物理削除を行う。

## 実装手順

### apply フローの変更

`app/src/service/apply.go` の IngressRoute 処理を以下のフローに変更する。

```
Step 7-3: IngressRoute を Kubernetes に反映する
├─ Project の IngressRoute を FindByProjectID で取得する
├─ IngressRoute が存在しない場合はスキップする
├─ FindActiveAndPendingByIngressRouteID で PathRule 一覧を取得する
├─ PathRule が 0 件の場合: k8s から IngressRoute を削除する
└─ PathRule が 1 件以上の場合:
   ├─ 各 PathRule の ServiceID から Service を取得し、Service 名とポートを解決する
   └─ k8s.ApplyIngressRoute() を PathRule 一覧とともに呼び出す

Step 10: pending 昇格・deleting 物理削除
├─ status=pending の PathRule を active に更新する
└─ status=deleting の PathRule を物理削除する
```

### k8s IngressRoute マニフェスト変更

`app/src/k8s/ingress_route.go` の `buildIngressRouteManifest` を複数ルート対応に変更する。

```go
type PathRuleSpec struct {
    PathPrefix  string // パスプレフィックス
    ServiceName string // Kubernetes Service 名
    ServicePort int    // Service ポート番号
}

func buildIngressRouteManifest(
    ingressRouteData *models.IngressRoute,
    namespace string,
    pathRuleSpecList []PathRuleSpec, // 複数パスルールを受け取る
) *unstructured.Unstructured {
    // routes を pathRuleSpecList の件数分生成する
    routeList := []interface{}{}
    for _, pathRuleSpec := range pathRuleSpecList {
        route := map[string]interface{}{
            "match": buildRouterRule(ingressRouteData.Host, pathRuleSpec.PathPrefix),
            "kind":  "Rule",
            "services": []interface{}{
                map[string]interface{}{
                    "name": pathRuleSpec.ServiceName,
                    "port": pathRuleSpec.ServicePort,
                },
            },
        }
        routeList = append(routeList, route) // ルートを追加する
    }
    // 以降は既存の manifest 構造に routeList を埋め込む
}
```

### apply の所属

IngressRoute の apply は Project apply の一部として実行する。
現状の Deployment apply に含まれていた IngressRoute apply を Project apply に移動する（または apply 対象を Project 単位に変更する）。

> **注意**: 現状 apply は Deployment 単位で実行されている。IngressRoute が Project に属するため、
> Project 内のいずれかの Deployment を apply したタイミングで Project の IngressRoute も反映するか、
> IngressRoute 専用の apply エンドポイントを設けるか、設計判断が必要。
> → **本 Issue では Deployment apply 時に同じ Project の IngressRoute も合わせて反映する形を採用する。**

### 変更ファイル

- `app/src/k8s/ingress_route.go`（編集）
    - 何を: `ApplyIngressRoute` を `[]PathRuleSpec` を受け取る形に変更し、複数ルートのマニフェストを生成する
    - なぜ: 1 IngressRoute に複数パスルールを反映するため

- `app/src/service/apply.go`（編集）
    - 何を: IngressRoute apply ロジックを PathRule 集約ベースに変更する。pending 昇格・deleting 物理削除を追加する
    - なぜ: PathRule テーブルの状態を Kubernetes に反映するため

## テスト確認項目

- [ ] PathRule が pending 状態で apply を実行すると Kubernetes に反映され status=active になること
- [ ] PathRule が複数ある場合、Traefik IngressRoute に複数ルートが生成されること
- [ ] status=deleting の PathRule が apply 後に物理削除されること
- [ ] PathRule が 0 件の場合、Kubernetes から IngressRoute が削除されること
- [ ] Service が pending 状態でも PathRule が正しく解決されること
