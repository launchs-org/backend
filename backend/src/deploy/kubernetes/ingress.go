package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// IngressRouteGVR は Traefik IngressRoute の GroupVersionResource です
var IngressRouteGVR = schema.GroupVersionResource{
	Group:    "traefik.io",
	Version:  "v1alpha1",
	Resource: "ingressroutes",
}

// IngressRouteParams は IngressRoute 作成に必要なパラメータです
type IngressRouteParams struct {
	Namespace   string
	Name        string
	Host        string
	PathPrefix  string
	ServiceName string
	ServicePort int32
	Middleware  string // 例: meecha-stripprefix
}

// ApplyIngressRoute は Traefik IngressRoute を作成または更新します
func ApplyIngressRoute(ctx context.Context, params IngressRouteParams) error {
	ingressRoute := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "IngressRoute",
			"metadata": map[string]interface{}{
				"name":      params.Name,
				"namespace": params.Namespace,
			},
			"spec": map[string]interface{}{
				"entryPoints": []interface{}{"web", "websecure"},
				"routes": []interface{}{
					map[string]interface{}{
						"match": fmt.Sprintf("Host(`%s`) && PathPrefix(`%s`)", params.Host, params.PathPrefix),
						"kind":  "Rule",
						"middlewares": []interface{}{
							map[string]interface{}{
								"name": params.Middleware,
							},
						},
						"services": []interface{}{
							map[string]interface{}{
								"name": params.ServiceName,
								"port": params.ServicePort,
							},
						},
					},
				},
			},
		},
	}

	resourceInterface := DynamicClient.Resource(IngressRouteGVR).Namespace(params.Namespace)

	// 既存のリソースを確認
	existing, err := resourceInterface.Get(ctx, params.Name, metav1.GetOptions{})
	if err != nil {
		// 作成
		_, err = resourceInterface.Create(ctx, ingressRoute, metav1.CreateOptions{})
		return err
	}

	// 更新 (ResourceVersion をセットする必要がある)
	ingressRoute.SetResourceVersion(existing.GetResourceVersion())
	_, err = resourceInterface.Update(ctx, ingressRoute, metav1.UpdateOptions{})
	return err
}

// DeleteIngressRoute は IngressRoute を削除します
func DeleteIngressRoute(ctx context.Context, namespace, name string) error {
	return DynamicClient.Resource(IngressRouteGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
