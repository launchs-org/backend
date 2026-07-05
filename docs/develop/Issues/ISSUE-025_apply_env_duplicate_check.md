# ISSUE-025 Apply拡張（環境変数適用・重複チェック）

## 親 Issue
ISSUE-021

## 概要
ApplyサービスにConfigMap・Secretの同期処理を追加する。apply時に重複するキー名が存在する場合はエラーとする。

## 変更ファイル一覧

- `app/src/service/apply.go`（編集）
    - **何を**: Applyメソッドの拡張。EnvVarMountsを取得してマウントされた環境変数を解決。is_secretによってConfigMapとSecretに分類。apply前にキー名重複チェックを実行（重複があればApplyHistoryをfailedにしてエラーを返す）。ConfigMapとSecretをk8sに適用。Deploymentのenv定義にConfigMap/Secretのenvfromを追加。
    - **なぜ**: k8sのDeploymentに環境変数を注入するためにConfigMap/Secretをk8s側で管理する必要があるため

## テスト確認項目

- [ ] applyでConfigMapとSecretが作成・更新されること
- [ ] Deploymentのenvにマウント設定が反映されること
- [ ] 重複キーが存在する場合にapplyがエラーになること
- [ ] ConfigMapのみ・Secretのみの場合も正常にapplyできること
