# ISSUE-029 k8s PVC操作

## 親 Issue
ISSUE-026

## 概要
k8s PersistentVolumeClaimのCRUD操作関数を実装する。

## 変更ファイル一覧

- `app/src/k8s/pvc.go`（新規作成）
    - **何を**: ApplyPVC（作成または更新）・DeletePVC関数の実装。VolumeモデルのsizeMBをGiに変換してPVCのstorageRequestに設定する。storage_classをStorageClassNameに設定する。
    - **なぜ**: k8s PVC操作の実装を集約するため

## テスト確認項目

- [ ] PVCが正常に作成されること
- [ ] size_mbが正しくPVCのstorageRequestに変換されること
- [ ] PVCが正常に削除されること
