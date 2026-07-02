package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

// newTestServiceManifest はテスト用の Service manifest を生成するヘルパー関数
func newTestServiceManifest(name, namespace string, serviceType corev1.ServiceType, port, targetPort int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name, // アプリ名ラベルを設定する
			},
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType, // Service タイプを設定する
			Selector: map[string]string{
				"app": name, // Deployment と一致するセレクターを設定する
			},
			Ports: []corev1.ServicePort{
				{
					Port:       port,                         // 公開ポートを設定する
					TargetPort: intstr.FromInt32(targetPort), // ターゲットポートを設定する
				},
			},
		},
	}
}

// TestApplyService_正常にServiceが作成される は ApplyService で k8s に Service が作成されることを確認する
func TestApplyService_正常にServiceが作成される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	serviceManifest := newTestServiceManifest("test-service", "test-namespace", corev1.ServiceTypeClusterIP, 8080, 3000) // テスト用 manifest を生成する

	err := ApplyService(ctx, fakeClient, serviceManifest) // Service を apply する
	if err != nil {
		t.Fatalf("ApplyService() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	createdService, err := fakeClient.CoreV1().Services("test-namespace").Get(ctx, "test-service", metav1.GetOptions{}) // 作成された Service を取得する
	if err != nil {
		t.Fatalf("Service の取得に失敗しました: %v", err) // 取得失敗時はテスト失敗とする
	}
	if createdService.Name != "test-service" { // Service 名を確認する
		t.Errorf("期待する Service 名: test-service, 実際: %s", createdService.Name)
	}
	if createdService.Spec.Type != corev1.ServiceTypeClusterIP { // Service タイプを確認する
		t.Errorf("期待する Service タイプ: ClusterIP, 実際: %s", createdService.Spec.Type)
	}
	if createdService.Spec.Ports[0].Port != 8080 { // ポート番号を確認する
		t.Errorf("期待するポート番号: 8080, 実際: %d", createdService.Spec.Ports[0].Port)
	}
}

// TestApplyService_同名Serviceを再applyすると更新される は同名の Service を再度 apply すると更新されることを確認する
func TestApplyService_同名Serviceを再applyすると更新される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	serviceManifest := newTestServiceManifest("update-service", "test-namespace", corev1.ServiceTypeClusterIP, 8080, 3000) // テスト用 manifest を生成する

	err := ApplyService(ctx, fakeClient, serviceManifest) // 1回目の apply（作成）
	if err != nil {
		t.Fatalf("1回目の ApplyService() がエラーを返しました: %v", err) // 1回目は成功するべきなのでテスト失敗とする
	}

	updatedManifest := newTestServiceManifest("update-service", "test-namespace", corev1.ServiceTypeClusterIP, 9090, 4000) // 更新用 manifest を生成する（ポートを変更）
	updatedManifest.Labels = map[string]string{"updated": "true", "app": "update-service"}                                 // ラベルを追加して更新内容を確認できるようにする

	err = ApplyService(ctx, fakeClient, updatedManifest) // 2回目の apply（更新）
	if err != nil {
		t.Fatalf("2回目の ApplyService() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	updatedService, err := fakeClient.CoreV1().Services("test-namespace").Get(ctx, "update-service", metav1.GetOptions{}) // 更新後の Service を取得する
	if err != nil {
		t.Fatalf("更新後の Service 取得に失敗しました: %v", err) // 取得失敗時はテスト失敗とする
	}
	if updatedService.Labels["updated"] != "true" { // 更新が反映されていることを確認する
		t.Errorf("Service の更新が反映されていません。Labels: %v", updatedService.Labels)
	}
	if updatedService.Spec.Ports[0].Port != 9090 { // ポート番号が更新されていることを確認する
		t.Errorf("期待するポート番号: 9090, 実際: %d", updatedService.Spec.Ports[0].Port)
	}
}

// TestDeleteService_正常にServiceが削除される は DeleteService で k8s から Service が削除されることを確認する
func TestDeleteService_正常にServiceが削除される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	serviceManifest := newTestServiceManifest("delete-service", "test-namespace", corev1.ServiceTypeClusterIP, 8080, 3000) // テスト用 manifest を生成する

	err := ApplyService(ctx, fakeClient, serviceManifest) // 削除対象の Service を作成する
	if err != nil {
		t.Fatalf("事前の ApplyService() がエラーを返しました: %v", err) // 前提条件の作成失敗時はテスト失敗とする
	}

	err = DeleteService(ctx, fakeClient, "test-namespace", "delete-service") // Service を削除する
	if err != nil {
		t.Fatalf("DeleteService() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	_, err = fakeClient.CoreV1().Services("test-namespace").Get(ctx, "delete-service", metav1.GetOptions{}) // 削除後に取得を試みる
	if err == nil {
		t.Fatal("削除後も Service が存在しています") // 削除後に取得できた場合はテスト失敗とする
	}
}
