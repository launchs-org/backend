package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ApplyConfigMap は k8s に ConfigMap を作成または更新する（命名規則: {deployName}-env）
func ApplyConfigMap(ctx context.Context, client kubernetes.Interface, namespace, deployName string, data map[string]string) error {
	configMapName := deployName + "-env" // ConfigMap 名を命名規則に従って生成する
	configMapManifest := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName, // ConfigMap 名を設定する
			Namespace: namespace,     // ネームスペースを設定する
		},
		Data: data, // 環境変数データを設定する
	}

	existing, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, configMapName, metav1.GetOptions{}) // 既存の ConfigMap を取得する
	if err != nil {
		// 存在しない場合は新規作成する
		_, err = client.CoreV1().ConfigMaps(namespace).Create(ctx, configMapManifest, metav1.CreateOptions{})
		return err
	}
	// 既存の ResourceVersion を設定して更新する（k8s の楽観的並行性制御のため）
	configMapManifest.ResourceVersion = existing.ResourceVersion
	_, err = client.CoreV1().ConfigMaps(namespace).Update(ctx, configMapManifest, metav1.UpdateOptions{})
	return err
}

// ApplySecret は k8s に Secret を作成または更新する（命名規則: {deployName}-secret）
func ApplySecret(ctx context.Context, client kubernetes.Interface, namespace, deployName string, data map[string][]byte) error {
	secretName := deployName + "-secret" // Secret 名を命名規則に従って生成する
	secretManifest := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName, // Secret 名を設定する
			Namespace: namespace,  // ネームスペースを設定する
		},
		Data: data, // シークレットデータを設定する
	}

	existing, err := client.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{}) // 既存の Secret を取得する
	if err != nil {
		// 存在しない場合は新規作成する
		_, err = client.CoreV1().Secrets(namespace).Create(ctx, secretManifest, metav1.CreateOptions{})
		return err
	}
	// 既存の ResourceVersion を設定して更新する（k8s の楽観的並行性制御のため）
	secretManifest.ResourceVersion = existing.ResourceVersion
	_, err = client.CoreV1().Secrets(namespace).Update(ctx, secretManifest, metav1.UpdateOptions{})
	return err
}

// DeleteConfigMap は k8s から ConfigMap を削除する（命名規則: {deployName}-env）
func DeleteConfigMap(ctx context.Context, client kubernetes.Interface, namespace, deployName string) error {
	configMapName := deployName + "-env"                                                  // ConfigMap 名を命名規則に従って生成する
	return client.CoreV1().ConfigMaps(namespace).Delete(ctx, configMapName, metav1.DeleteOptions{}) // ConfigMap を削除する
}

// DeleteSecret は k8s から Secret を削除する（命名規則: {deployName}-secret）
func DeleteSecret(ctx context.Context, client kubernetes.Interface, namespace, deployName string) error {
	secretName := deployName + "-secret"                                                 // Secret 名を命名規則に従って生成する
	return client.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{}) // Secret を削除する
}
