package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestApplyPVC_正常にPVCが作成される は ApplyPVC で k8s に PVC が作成されることを確認する
func TestApplyPVC_正常にPVCが作成される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	pvcManifest := BuildPVCManifest("test-namespace", "test-pvc", 1024, "") // テスト用 PVC manifest を生成する

	err := ApplyPVC(ctx, fakeClient, pvcManifest) // PVC を apply する
	if err != nil {
		t.Fatalf("ApplyPVC() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	createdPVC, err := fakeClient.CoreV1().PersistentVolumeClaims("test-namespace").Get(ctx, "test-pvc", metav1.GetOptions{}) // 作成された PVC を取得する
	if err != nil {
		t.Fatalf("PVC の取得に失敗しました: %v", err) // 取得失敗時はテスト失敗とする
	}
	if createdPVC.Name != "test-pvc" { // PVC 名を確認する
		t.Errorf("期待する PVC 名: test-pvc, 実際: %s", createdPVC.Name)
	}
}

// TestApplyPVC_sizeMBがMiB単位でPVCに設定される は SizeMB が正しく MiB 単位に変換されて PVC に設定されることを確認する
func TestApplyPVC_sizeMBがMiB単位でPVCに設定される(t *testing.T) {
	testCases := []struct {
		name            string
		sizeMB          int
		expectedQuantity string
	}{
		{
			name:            "1024MBは1024Miになる",    // 1024MB のテストケース
			sizeMB:          1024,
			expectedQuantity: "1024Mi",
		},
		{
			name:            "2048MBは2048Miになる",    // 2048MB のテストケース
			sizeMB:          2048,
			expectedQuantity: "2048Mi",
		},
		{
			name:            "512MBは512Miになる",      // 512MB のテストケース
			sizeMB:          512,
			expectedQuantity: "512Mi",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			pvcManifest := BuildPVCManifest("test-namespace", "test-pvc", testCase.sizeMB, "") // PVC manifest を生成する

			expectedQuantity := resource.MustParse(testCase.expectedQuantity)                                            // 期待する Quantity を生成する
			actualQuantity := pvcManifest.Spec.Resources.Requests[corev1.ResourceStorage]                               // 実際の Quantity を取得する
			if actualQuantity.Cmp(expectedQuantity) != 0 {                                                               // Quantity を比較する
				t.Errorf("sizeMB=%d: 期待する storage: %s, 実際: %s", testCase.sizeMB, expectedQuantity.String(), actualQuantity.String())
			}
		})
	}
}

// TestApplyPVC_StorageClassNameが設定される は StorageClassName が正しく PVC に設定されることを確認する
func TestApplyPVC_StorageClassNameが設定される(t *testing.T) {
	storageClassName := "standard"                                                     // テスト用 StorageClass 名
	pvcManifest := BuildPVCManifest("test-namespace", "test-pvc", 1024, storageClassName) // StorageClass 指定で manifest を生成する

	if pvcManifest.Spec.StorageClassName == nil { // StorageClassName が設定されていることを確認する
		t.Fatal("StorageClassName が nil です")
	}
	if *pvcManifest.Spec.StorageClassName != storageClassName { // StorageClassName の値を確認する
		t.Errorf("期待する StorageClassName: %s, 実際: %s", storageClassName, *pvcManifest.Spec.StorageClassName)
	}
}

// TestApplyPVC_同名PVCを再applyするとスキップされる は同名の PVC を再度 apply してもエラーにならずスキップされることを確認する
// PVC の spec は Bound 後 immutable なため、既存 PVC が存在する場合は更新せずそのまま返す設計
func TestApplyPVC_同名PVCを再applyするとスキップされる(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	pvcManifest := BuildPVCManifest("test-namespace", "update-pvc", 1024, "") // テスト用 PVC manifest を生成する
	pvcManifest.Labels = map[string]string{"original": "true"}                // 元のラベルを設定する

	err := ApplyPVC(ctx, fakeClient, pvcManifest) // 1回目の apply（作成）
	if err != nil {
		t.Fatalf("1回目の ApplyPVC() がエラーを返しました: %v", err) // 前提条件の作成失敗時はテスト失敗とする
	}

	updatedManifest := BuildPVCManifest("test-namespace", "update-pvc", 1024, "") // 更新用 manifest を生成する
	updatedManifest.Labels = map[string]string{"updated": "true"}                 // 別のラベルを設定する

	err = ApplyPVC(ctx, fakeClient, updatedManifest) // 2回目の apply（既存あり → スキップ）
	if err != nil {
		t.Fatalf("2回目の ApplyPVC() がエラーを返しました: %v", err) // スキップ時にエラーが返ってはいけない
	}

	existingPVC, err := fakeClient.CoreV1().PersistentVolumeClaims("test-namespace").Get(ctx, "update-pvc", metav1.GetOptions{}) // 取得して内容を確認する
	if err != nil {
		t.Fatalf("PVC の取得に失敗しました: %v", err) // 取得失敗時はテスト失敗とする
	}
	if existingPVC.Labels["updated"] == "true" { // ラベルが更新されていないことを確認する（spec immutable のためスキップされる）
		t.Errorf("既存 PVC が上書きされています。スキップされるべきです")
	}
	if existingPVC.Labels["original"] != "true" { // 元のラベルが保持されていることを確認する
		t.Errorf("元のラベルが失われています。Labels: %v", existingPVC.Labels)
	}
}

// TestDeletePVC_正常にPVCが削除される は DeletePVC で k8s から PVC が削除されることを確認する
func TestDeletePVC_正常にPVCが削除される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	pvcManifest := BuildPVCManifest("test-namespace", "delete-pvc", 1024, "") // 削除対象の PVC manifest を生成する

	err := ApplyPVC(ctx, fakeClient, pvcManifest) // 削除対象の PVC を作成する
	if err != nil {
		t.Fatalf("事前の ApplyPVC() がエラーを返しました: %v", err) // 前提条件の作成失敗時はテスト失敗とする
	}

	err = DeletePVC(ctx, fakeClient, "test-namespace", "delete-pvc") // PVC を削除する
	if err != nil {
		t.Fatalf("DeletePVC() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	_, err = fakeClient.CoreV1().PersistentVolumeClaims("test-namespace").Get(ctx, "delete-pvc", metav1.GetOptions{}) // 削除後に取得を試みる
	if err == nil {
		t.Fatal("削除後も PVC が存在しています") // 削除後に取得できた場合はテスト失敗とする
	}
}
