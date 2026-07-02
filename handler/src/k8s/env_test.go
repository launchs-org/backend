package k8s

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestApplyConfigMap_正常にConfigMapが作成される は ApplyConfigMap で k8s に ConfigMap が作成されることを確認する
func TestApplyConfigMap_正常にConfigMapが作成される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する
	configData := map[string]string{
		"APP_ENV": "production", // テスト用環境変数を定義する
		"PORT":    "8080",       // テスト用ポート番号を定義する
	}

	err := ApplyConfigMap(ctx, fakeClient, "test-namespace", "my-deploy", configData) // ConfigMap を apply する
	if err != nil {
		t.Fatalf("ApplyConfigMap() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	createdConfigMap, err := fakeClient.CoreV1().ConfigMaps("test-namespace").Get(ctx, "my-deploy-env", metav1.GetOptions{}) // 作成された ConfigMap を取得する
	if err != nil {
		t.Fatalf("ConfigMap の取得に失敗しました: %v", err) // 取得失敗時はテスト失敗とする
	}
	if createdConfigMap.Name != "my-deploy-env" { // ConfigMap 名を確認する
		t.Errorf("期待する ConfigMap 名: my-deploy-env, 実際: %s", createdConfigMap.Name)
	}
	if createdConfigMap.Data["APP_ENV"] != "production" { // データが正しく格納されているか確認する
		t.Errorf("期待する APP_ENV: production, 実際: %s", createdConfigMap.Data["APP_ENV"])
	}
}

// TestApplyConfigMap_同名ConfigMapを再applyすると更新される は同名の ConfigMap を再度 apply すると更新されることを確認する
func TestApplyConfigMap_同名ConfigMapを再applyすると更新される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	initialData := map[string]string{"KEY": "initial"} // 初期データを定義する
	err := ApplyConfigMap(ctx, fakeClient, "test-namespace", "update-deploy", initialData) // 1回目の apply（作成）
	if err != nil {
		t.Fatalf("1回目の ApplyConfigMap() がエラーを返しました: %v", err) // 1回目は成功するべきなのでテスト失敗とする
	}

	updatedData := map[string]string{"KEY": "updated", "NEW_KEY": "value"} // 更新データを定義する
	err = ApplyConfigMap(ctx, fakeClient, "test-namespace", "update-deploy", updatedData) // 2回目の apply（更新）
	if err != nil {
		t.Fatalf("2回目の ApplyConfigMap() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	updatedConfigMap, err := fakeClient.CoreV1().ConfigMaps("test-namespace").Get(ctx, "update-deploy-env", metav1.GetOptions{}) // 更新後の ConfigMap を取得する
	if err != nil {
		t.Fatalf("更新後の ConfigMap 取得に失敗しました: %v", err) // 取得失敗時はテスト失敗とする
	}
	if updatedConfigMap.Data["KEY"] != "updated" { // 更新が反映されていることを確認する
		t.Errorf("期待する KEY: updated, 実際: %s", updatedConfigMap.Data["KEY"])
	}
	if updatedConfigMap.Data["NEW_KEY"] != "value" { // 新規キーが追加されていることを確認する
		t.Errorf("期待する NEW_KEY: value, 実際: %s", updatedConfigMap.Data["NEW_KEY"])
	}
}

// TestDeleteConfigMap_正常にConfigMapが削除される は DeleteConfigMap で k8s から ConfigMap が削除されることを確認する
func TestDeleteConfigMap_正常にConfigMapが削除される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	configData := map[string]string{"KEY": "value"} // テスト用データを定義する
	err := ApplyConfigMap(ctx, fakeClient, "test-namespace", "delete-deploy", configData) // 削除対象の ConfigMap を作成する
	if err != nil {
		t.Fatalf("事前の ApplyConfigMap() がエラーを返しました: %v", err) // 前提条件の作成失敗時はテスト失敗とする
	}

	err = DeleteConfigMap(ctx, fakeClient, "test-namespace", "delete-deploy") // ConfigMap を削除する
	if err != nil {
		t.Fatalf("DeleteConfigMap() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	_, err = fakeClient.CoreV1().ConfigMaps("test-namespace").Get(ctx, "delete-deploy-env", metav1.GetOptions{}) // 削除後に取得を試みる
	if err == nil {
		t.Fatal("削除後も ConfigMap が存在しています") // 削除後に取得できた場合はテスト失敗とする
	}
}

// TestApplySecret_正常にSecretが作成される は ApplySecret で k8s に Secret が作成されることを確認する
func TestApplySecret_正常にSecretが作成される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する
	secretData := map[string][]byte{
		"DB_PASSWORD": []byte("secret123"), // テスト用シークレットデータを定義する
		"API_KEY":     []byte("abcdef"),    // テスト用 API キーを定義する
	}

	err := ApplySecret(ctx, fakeClient, "test-namespace", "my-deploy", secretData) // Secret を apply する
	if err != nil {
		t.Fatalf("ApplySecret() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	createdSecret, err := fakeClient.CoreV1().Secrets("test-namespace").Get(ctx, "my-deploy-secret", metav1.GetOptions{}) // 作成された Secret を取得する
	if err != nil {
		t.Fatalf("Secret の取得に失敗しました: %v", err) // 取得失敗時はテスト失敗とする
	}
	if createdSecret.Name != "my-deploy-secret" { // Secret 名を確認する
		t.Errorf("期待する Secret 名: my-deploy-secret, 実際: %s", createdSecret.Name)
	}
	if string(createdSecret.Data["DB_PASSWORD"]) != "secret123" { // データが正しく格納されているか確認する
		t.Errorf("期待する DB_PASSWORD: secret123, 実際: %s", string(createdSecret.Data["DB_PASSWORD"]))
	}
}

// TestApplySecret_同名Secretを再applyすると更新される は同名の Secret を再度 apply すると更新されることを確認する
func TestApplySecret_同名Secretを再applyすると更新される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	initialData := map[string][]byte{"PASSWORD": []byte("old")} // 初期データを定義する
	err := ApplySecret(ctx, fakeClient, "test-namespace", "update-deploy", initialData) // 1回目の apply（作成）
	if err != nil {
		t.Fatalf("1回目の ApplySecret() がエラーを返しました: %v", err) // 1回目は成功するべきなのでテスト失敗とする
	}

	updatedData := map[string][]byte{"PASSWORD": []byte("new"), "TOKEN": []byte("xyz")} // 更新データを定義する
	err = ApplySecret(ctx, fakeClient, "test-namespace", "update-deploy", updatedData) // 2回目の apply（更新）
	if err != nil {
		t.Fatalf("2回目の ApplySecret() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	updatedSecret, err := fakeClient.CoreV1().Secrets("test-namespace").Get(ctx, "update-deploy-secret", metav1.GetOptions{}) // 更新後の Secret を取得する
	if err != nil {
		t.Fatalf("更新後の Secret 取得に失敗しました: %v", err) // 取得失敗時はテスト失敗とする
	}
	if string(updatedSecret.Data["PASSWORD"]) != "new" { // 更新が反映されていることを確認する
		t.Errorf("期待する PASSWORD: new, 実際: %s", string(updatedSecret.Data["PASSWORD"]))
	}
	if string(updatedSecret.Data["TOKEN"]) != "xyz" { // 新規キーが追加されていることを確認する
		t.Errorf("期待する TOKEN: xyz, 実際: %s", string(updatedSecret.Data["TOKEN"]))
	}
}

// TestDeleteSecret_正常にSecretが削除される は DeleteSecret で k8s から Secret が削除されることを確認する
func TestDeleteSecret_正常にSecretが削除される(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // fake k8s クライアントを生成する
	ctx := context.Background()            // テスト用コンテキストを生成する

	secretData := map[string][]byte{"KEY": []byte("value")} // テスト用データを定義する
	err := ApplySecret(ctx, fakeClient, "test-namespace", "delete-deploy", secretData) // 削除対象の Secret を作成する
	if err != nil {
		t.Fatalf("事前の ApplySecret() がエラーを返しました: %v", err) // 前提条件の作成失敗時はテスト失敗とする
	}

	err = DeleteSecret(ctx, fakeClient, "test-namespace", "delete-deploy") // Secret を削除する
	if err != nil {
		t.Fatalf("DeleteSecret() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}

	_, err = fakeClient.CoreV1().Secrets("test-namespace").Get(ctx, "delete-deploy-secret", metav1.GetOptions{}) // 削除後に取得を試みる
	if err == nil {
		t.Fatal("削除後も Secret が存在しています") // 削除後に取得できた場合はテスト失敗とする
	}
}
