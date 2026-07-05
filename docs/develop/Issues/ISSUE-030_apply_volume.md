# ISSUE-030 Apply拡張（PVC対応）

## 親 Issue
ISSUE-026

## 概要
ApplyサービスにPVCの同期処理とDeploymentへのvolumeMountsの追加を実装する。

## 変更ファイル一覧

- `app/src/service/apply.go`（編集）
    - **何を**: Applyメソッドの拡張。VolumeMountsを取得して参照するVolumeを解決。各VolumeのPVCをk8sに適用。Deploymentのvolumes・volumeMountsにマウント設定を追加。apply成功後にVolumeMountのstatusをappliedに更新。
    - **なぜ**: PersistentストレージをDeploymentから利用するためにPVCとvolumeMountsを同期する必要があるため

## テスト確認項目

- [ ] applyでPVCが作成・更新されること
- [ ] DeploymentのvolumeMountsにマウント設定が反映されること
- [ ] PVC作成失敗時にApplyHistoryがfailedに更新されること
