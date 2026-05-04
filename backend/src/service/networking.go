package service

import (
	"context"
	"crypto/sha256"
	"github.com/btcsuite/btcutil/base58"

	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"backend/database"
	"backend/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// ServicePort はサービスポートの設定を表す構造体です
type ServicePort struct {
	Name   string `json:"name"`   // ポート名
	Port   int    `json:"port"`   // 外部ポート
	Target int    `json:"target"` // 内部ポート(コンテナ側)
}

// UpdateService はサービスの設定を更新し、Kubernetesリソースを同期します
func UpdateService(ctx context.Context, containerID, ownerID, svcType string, ports []ServicePort, isActive bool) (map[string]interface{}, error) {
	// 1. コンテナを取得
	container, err := model.GetContainerByID(containerID)
	// エラーがある場合
	if err != nil {
		// レコードが見つからない場合
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// コンテナが見つからないエラーを返す
			return nil, ErrContainerNotFound
		}
		// その他のエラーを返す
		return nil, err
	}

	// 2. プロジェクトを取得して権限チェック
	project, err := model.GetProjectByID(container.ProjectID)
	// エラーがある場合
	if err != nil {
		// エラーを返す
		return nil, err
	}
	// 所有者が一致しない場合
	if project.OwnerID != ownerID {
		// 権限エラーを返す
		return nil, ErrForbidden
	}

	// 3. DBのServiceレコードを取得
	svc, err := model.GetServiceByContainerID(containerID)
	// エラーがある場合
	if err != nil {
		// エラーを返す
		return nil, err
	}

	// 4. ポート設定をJSONに変換
	portsJSON, err := json.Marshal(ports)
	// エラーがある場合
	if err != nil {
		// エラーを返す
		return nil, fmt.Errorf("failed to marshal ports: %w", err)
	}

	// 5. DBレコードを更新
	svc.Type = svcType
	svc.Ports = string(portsJSON)
	svc.IsActive = isActive
	// モデルの更新関数を呼び出す
	if err := model.UpdateService(svc); err != nil {
		// エラーを返す
		return nil, err
	}

	// 6. Kubernetes Serviceを同期
	if !isActive || len(ports) == 0 {
		// 非アクティブまたはポートが空の場合はK8s Serviceを削除してIPをクリア
		_ = deleteK8sService(ctx, project.Namespace, container.Name)
		svc.InternalIP = ""
		svc.ExternalIP = ""
		// モデルの更新関数を呼び出してIPをクリア保存
		if err := model.UpdateService(svc); err != nil {
			// エラーを返す
			return nil, err
		}
		// 成功を返す
		return map[string]interface{}{
			"data": svc,
		}, nil
	}

	// アクティブかつポートがある場合は同期を実行
	k8sSvcObj, err := syncK8sService(ctx, project.Namespace, container.Name, svcType, ports)
	if err != nil {
		// エラーを返す
		return nil, fmt.Errorf("failed to sync k8s service: %w", err)
	}

	// 7. IP情報をDBレコードに反映
	svc.InternalIP = k8sSvcObj.Spec.ClusterIP
	if len(k8sSvcObj.Status.LoadBalancer.Ingress) > 0 {
		svc.ExternalIP = k8sSvcObj.Status.LoadBalancer.Ingress[0].IP
		if svc.ExternalIP == "" {
			svc.ExternalIP = k8sSvcObj.Status.LoadBalancer.Ingress[0].Hostname
		}
	}

	// モデルの更新関数を再度呼び出してIPを保存
	if err := model.UpdateService(svc); err != nil {
		// エラーを返す
		return nil, err
	}

	// 8. 更新後の情報を返す
	return map[string]interface{}{
		"data": svc,
	}, nil
}

// deleteK8sService はKubernetes上のServiceリソースを削除します
func deleteK8sService(ctx context.Context, namespace, name string) error {
	// K8sクライアントを取得
	clientset := database.K8sClientset.(*kubernetes.Clientset)
	// 削除を実行
	return clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// syncK8sService はKubernetes上のServiceリソースを同期します
func syncK8sService(ctx context.Context, namespace, name, svcType string, ports []ServicePort) (*corev1.Service, error) {
	// K8sクライアントを取得
	clientset := database.K8sClientset.(*kubernetes.Clientset)

	// K8s用のポート設定に変換
	var k8sPorts []corev1.ServicePort
	// 各ポート設定をループ
	for _, p := range ports {
		// K8s ServicePort構造体を作成
		k8sPorts = append(k8sPorts, corev1.ServicePort{
			Name:       p.Name,
			Port:       int32(p.Port),
			TargetPort: intstr.FromInt(p.Target),
		})
	}

	// Serviceオブジェクトを定義
	k8sSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceType(svcType),
			Selector: map[string]string{"app": name},
			Ports:    k8sPorts,
		},
	}

	// Serviceクライアントを取得
	svcClient := clientset.CoreV1().Services(namespace)
	// 既存のServiceを確認
	existing, err := svcClient.Get(ctx, name, metav1.GetOptions{})
	// エラーがある場合
	if err != nil {
		// 作成を試みる
		return svcClient.Create(ctx, k8sSvc, metav1.CreateOptions{})
	}

	// 既存のリソースを更新
	existing.Spec.Type = k8sSvc.Spec.Type
	existing.Spec.Ports = k8sSvc.Spec.Ports
	// 更新を実行
	return svcClient.Update(ctx, existing, metav1.UpdateOptions{})
}

// CreateIngress はIngressを作成し、Kubernetesリソースを同期します
func CreateIngress(ctx context.Context, containerID, ownerID, customDomain string, customDomainEnabled bool, httpPort int) (map[string]interface{}, error) {
	// 1. コンテナを取得
	container, err := model.GetContainerByID(containerID)
	// エラーがある場合
	if err != nil {
		// エラーを返す
		return nil, err
	}

	// 2. プロジェクトを取得して権限チェック
	project, err := model.GetProjectByID(container.ProjectID)
	// エラーがある場合
	if err != nil {
		// エラーを返す
		return nil, err
	}
	// 所有者が一致しない場合
	if project.OwnerID != ownerID {
		// 権限エラーを返す
		return nil, ErrForbidden
	}

	// 3. 既存のIngressを確認
	existingIng, _ := model.GetIngressByContainerID(containerID)
	// すでに存在する場合
	if existingIng != nil {
		// 重複エラーを返す
		return nil, errors.New("ingress already exists for this container")
	}

	// sha256にする
	hasher := sha256.Sum256([]byte(fmt.Sprintf("%s-%s", project.ID, container.ID)))
	//sha256をbase58に変換
	encoded := base58.Encode(hasher[:])

	// 4. サブドメインを生成 (例: {project}-{container}.launchs.org)
	subdomain := fmt.Sprintf("%s.launchs.org", encoded)

	// 5. DBにレコードを作成
	ing := &model.Ingress{
		ID:                  "ing_" + uuid.New().String(),
		ContainerID:         containerID,
		Subdomain:           subdomain,
		CustomDomain:        customDomain,
		CustomDomainEnabled: customDomainEnabled,
		HttpPort:            httpPort,
		TlsEnabled:          false, // 固定
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	// モデルの作成関数を呼び出す
	if err := model.CreateIngress(ing); err != nil {
		// エラーを返す
		return nil, err
	}

	// 6. Kubernetes Ingressを同期
	hosts := []string{subdomain}
	if customDomain != "" && customDomainEnabled {
		hosts = append(hosts, customDomain)
	}
	if err := syncK8sIngressRoute(ctx, project.Namespace, container.Name, hosts, "/", httpPort); err != nil {
		// エラーを返す
		return nil, fmt.Errorf("failed to sync k8s ingress: %w", err)
	}

	// 7. 作成した情報を返す
	return map[string]interface{}{
		"data": ing,
	}, nil
}

// UpdateIngress はIngress設定を更新し、Kubernetesリソースを同期します
func UpdateIngress(ctx context.Context, containerID, ownerID, customDomain string, customDomainEnabled bool, httpPort int) (map[string]interface{}, error) {
	// 1. コンテナを取得
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		return nil, err
	}

	// 2. プロジェクトを取得して権限チェック
	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.OwnerID != ownerID {
		return nil, ErrForbidden
	}

	// 3. DBから既存のIngressを取得
	ing, err := model.GetIngressByContainerID(containerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("ingress not found")
		}
		return nil, err
	}

	// 4. フィールドを更新
	ing.CustomDomain = customDomain
	ing.CustomDomainEnabled = customDomainEnabled
	ing.HttpPort = httpPort
	ing.UpdatedAt = time.Now()

	// 5. DBを更新
	if err := model.UpdateIngress(ing); err != nil {
		return nil, err
	}

	// 6. Kubernetes Ingressを同期
	hosts := []string{ing.Subdomain}
	if ing.CustomDomain != "" && ing.CustomDomainEnabled {
		hosts = append(hosts, ing.CustomDomain)
	}
	if err := syncK8sIngressRoute(ctx, project.Namespace, container.Name, hosts, "/", ing.HttpPort); err != nil {
		return nil, fmt.Errorf("failed to sync k8s ingress: %w", err)
	}

	// 7. 更新した情報を返す
	return map[string]interface{}{
		"data": ing,
	}, nil
}

func syncK8sIngressRoute(ctx context.Context, namespace, name string, hosts []string, path string, port int) error {
	// Dynamic Clientを取得 (事前に database.DynamicClient として初期化しておく)
	client := database.K8sDynamicClient // dynamic.Interface

	// IngressRoute のリソース定義 (GVR)
	resourceGVR := schema.GroupVersionResource{
		Group:    "traefik.io",
		Version:  "v1alpha1",
		Resource: "ingressroutes",
	}

	// ホストのマッチルールを組み立て
	var hostMatches []string
	for _, host := range hosts {
		hostMatches = append(hostMatches, fmt.Sprintf("Host(`%s`)", host))
	}
	// ホストが複数ある場合は OR (||) で繋ぐ
	matchRule := ""
	if len(hostMatches) > 1 {
		matchRule = fmt.Sprintf("(%s) && PathPrefix(`%s`)", strings.Join(hostMatches, " || "), path)
	} else if len(hostMatches) == 1 {
		matchRule = fmt.Sprintf("%s && PathPrefix(`%s`)", hostMatches[0], path)
	}

	// Unstructured (汎用構造体) で IngressRoute を組み立て
	ingressRoute := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "IngressRoute",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"entryPoints": []interface{}{"web", "websecure"},
				"routes": []interface{}{
					map[string]interface{}{
						"match": matchRule,
						"kind":  "Rule",
						"services": []interface{}{
							map[string]interface{}{
								"name": name,
								"port": port,
							},
						},
					},
				},
			},
		},
	}

	// 既存確認
	res, err := client.Resource(resourceGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// 新規作成
		_, err = client.Resource(resourceGVR).Namespace(namespace).Create(ctx, ingressRoute, metav1.CreateOptions{})
		return err
	}

	// 更新 (ResourceVersionをセットしてUpdate)
	ingressRoute.SetResourceVersion(res.GetResourceVersion())
	_, err = client.Resource(resourceGVR).Namespace(namespace).Update(ctx, ingressRoute, metav1.UpdateOptions{})
	return err
}

// DeleteIngress はIngressを削除し、Kubernetesリソースを同期します
// DeleteIngressRoute は Traefik の IngressRoute を削除し、DBレコードを同期します
func DeleteIngressRoute(ctx context.Context, containerID, ownerID string) (map[string]interface{}, error) {
	// 1. コンテナを取得
	container, err := model.GetContainerByID(containerID)
	if err != nil {
		return nil, err
	}

	// 2. プロジェクトを取得して権限チェック
	project, err := model.GetProjectByID(container.ProjectID)
	if err != nil {
		return nil, err
	}
	// 所有者が一致しない場合
	if project.OwnerID != ownerID {
		return nil, errors.New("forbidden: you do not own this project")
	}

	// 3. DBからIngress（IngressRoute）の情報を取得
	ing, err := model.GetIngressByContainerID(containerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("ingressroute record not found in database")
		}
		return nil, err
	}

	// 4. Kubernetes IngressRoute (Traefik CRD) を削除
	// database.DynamicClient は dynamic.Interface として初期化されている前提
	dynClient := database.K8sDynamicClient

	// IngressRoute のリソース定義 (Group, Version, Resource)
	resourceGVR := schema.GroupVersionResource{
		Group:    "traefik.io",
		Version:  "v1alpha1",
		Resource: "ingressroutes",
	}

	// Traefik の IngressRoute を削除
	// 名前は container.Name を使用（作成時と合わせる必要があります）
	err = dynClient.Resource(resourceGVR).Namespace(project.Namespace).Delete(ctx, container.Name, metav1.DeleteOptions{})
	if err != nil {
		// すでに存在しない場合はエラーにせずログ出力のみにする（冪等性の確保）
		fmt.Printf("Warning: failed to delete k8s ingressroute (it might be already deleted): %v\n", err)
	}

	// 5. DBレコードを削除
	if err := model.DeleteIngress(containerID); err != nil {
		return nil, err
	}

	// 6. 削除したIDを返す
	return map[string]interface{}{
		"data": map[string]string{"id": ing.ID},
	}, nil
}
