# ISSUE-011 k8s Manifestジェネレーター

## 親 Issue
ISSUE-009

## 概要
DBのDeploymentモデルからk8s Deployment manifestを生成する関数を実装する。InstanceSizeマスタからCPU・メモリのリソース制限値を解決する。

## 変更ファイル一覧

- `app/src/k8s/manifest/generator.go`（編集）
    - **何を**: GenerateDeployment関数の実装。DeploymentモデルとInstanceSizeマスタ・namespace・replicasを受け取り、apps/v1 Deploymentオブジェクトを返す。リソースリクエスト・リミット、ラベル（launchs.org/deployment-id）、コマンド・argsのオーバーライドを設定する。
    - **なぜ**: DBモデルからk8s APIオブジェクトへの変換ロジックを集約するため

- `app/src/models/instance_size.go`（編集）
    - **何を**: InstanceSizeモデルの定義。small/medium/largeなどのサイズ名、CPU・メモリのrequest/limitフィールドを持つ。
    - **なぜ**: インスタンスサイズのマスタデータをDBで管理するため

## テスト確認項目

- [ ] DeploymentモデルからInstanceSizeが解決されてk8s Deploymentが生成されること
- [ ] コマンド・argsが指定された場合にManifestに反映されること
- [ ] ラベルにdeployment-idが設定されること
- [ ] replicasが正しく設定されること
