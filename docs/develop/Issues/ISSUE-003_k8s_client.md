# ISSUE-003 Kubernetesクライアント初期化

## 親 Issue
ISSUE-001

## 概要
In-cluster configまたはkubeconfigからKubernetesクライアントを初期化する。標準クライアント（kubernetes.Clientset）とdynamicクライアント（dynamic.Interface）の両方を用意する。

## 変更ファイル一覧

- `app/src/k8s/client.go`（編集）
    - **何を**: NewK8sClient()関数の実装。In-cluster config優先、失敗時はkubeconfigにフォールバック。kubernetes.Clientsetとdynamic.Interfaceを返す。
    - **なぜ**: k8s APIへのアクセスにはクライアント初期化が必要であり、環境（本番/開発）によって設定方法が異なるため
- `app/src/main.go`（編集）
    - **何を**: k8s.NewK8sClient()の呼び出しと、返却されたクライアントをハンドラーのDIに渡す処理の追加。
    - **なぜ**: k8sクライアントをサービス層に注入するため

## テスト確認項目

- [ ] kubeconfigが存在する環境でクライアントが初期化できること
- [ ] In-cluster環境でクライアントが初期化できること
