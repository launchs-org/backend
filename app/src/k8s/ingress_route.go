package k8s

import (
	"app/logger"
	"app/models"
	"app/repository"
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"gorm.io/datatypes"
)

// traefikIngressRouteGVR は Traefik IngressRoute CRD の GroupVersionResource を定義する
var traefikIngressRouteGVR = schema.GroupVersionResource{
	Group:    "traefik.io",
	Version:  "v1alpha1",
	Resource: "ingressroutes",
}

// traefikMiddlewareGVR は Traefik Middleware CRD の GroupVersionResource を定義する
var traefikMiddlewareGVR = schema.GroupVersionResource{
	Group:    "traefik.io",
	Version:  "v1alpha1",
	Resource: "middlewares",
}

// middlewareName は IngressRoute ID から StripPrefix Middleware の名前を生成する
func middlewareName(ingressRouteID string) string {
	return fmt.Sprintf("strip-%s", ingressRouteID) // strip-{ingressRouteID} 形式で名前を生成する
}

// buildRouterRule は IngressRoute のルールマッチ文字列を生成する
func buildRouterRule(host, pathPrefix string) string {
	return fmt.Sprintf("Host(`%s`) && PathPrefix(`%s`)", host, pathPrefix) // ホストとパスプレフィックスのルールを生成する
}

// PathRuleSpec は IngressRoute の1つのパスルールを表す
type PathRuleSpec struct {
	PathPrefix  string // ルーティング対象パスプレフィックス
	ServiceName string // 転送先 Kubernetes Service 名
	ServicePort int    // 転送先 Service ポート番号
	StripPrefix bool   // パスプレフィックスを strip するか
}

// buildIngressRouteManifest は Traefik IngressRoute の unstructured マニフェストを生成する
func buildIngressRouteManifest(ingressRouteData models.IngressRoute, namespace string, pathRuleSpecList []PathRuleSpec) *unstructured.Unstructured {
	middlewareRef := fmt.Sprintf("%s@kubernetescrd", middlewareName(ingressRouteData.ID)) // Middleware 参照文字列を生成する
	routeList := make([]interface{}, 0, len(pathRuleSpecList))                            // ルート一覧を初期化する
	for _, pathRuleSpec := range pathRuleSpecList {                                       // PathRuleSpec ごとに Traefik ルートを生成する
		routeRule := buildRouterRule(ingressRouteData.Host, pathRuleSpec.PathPrefix) // ルールを生成する
		routeEntry := map[string]interface{}{
			"kind":  "Rule",
			"match": routeRule, // ルール文字列を設定する
			"services": []interface{}{
				map[string]interface{}{
					"name": pathRuleSpec.ServiceName,        // サービス名を設定する
					"port": int64(pathRuleSpec.ServicePort), // ポートを設定する
				},
			},
		}
		if pathRuleSpec.StripPrefix { // strip_prefix が有効な場合は Middleware 参照を追加する
			routeEntry["middlewares"] = []interface{}{
				map[string]interface{}{
					"name": middlewareRef, // Middleware 名を設定する
				},
			}
		}
		routeList = append(routeList, routeEntry) // ルートを追加する
	}

	spec := map[string]interface{}{
		"entryPoints": []interface{}{"web", "websecure"}, // エントリーポイントを設定する
		"routes":      routeList,                         // 複数ルートを設定する
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "IngressRoute",
			"metadata": map[string]interface{}{
				"name":      ingressRouteData.ID, // IngressRoute 名を設定する（IngressRoute ID を使う）
				"namespace": namespace,           // namespace を設定する
				"labels": map[string]interface{}{
					"launchs-managed": "true", // launchs が管理するリソースであることを示すラベル
					"generated":       "1",    // 自動生成リソースであることを示すラベル
				},
			},
			"spec": spec,
		},
	}
}

// buildMiddlewareManifest は Traefik StripPrefix Middleware の unstructured マニフェストを生成する
func buildMiddlewareManifest(ingressRouteID string, namespace string, prefixList []string) *unstructured.Unstructured {
	prefixInterface := make([]interface{}, 0, len(prefixList)) // interface スライスに変換する
	for _, prefix := range prefixList {
		prefixInterface = append(prefixInterface, prefix) // 各プレフィックスを追加する
	}
	name := middlewareName(ingressRouteID) // Middleware 名を生成する
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "Middleware",
			"metadata": map[string]interface{}{
				"name":      name,      // Middleware 名を設定する
				"namespace": namespace, // namespace を設定する
				"labels": map[string]interface{}{
					"launchs-managed": "true", // launchs が管理するリソースであることを示すラベル
				},
			},
			"spec": map[string]interface{}{
				"stripPrefix": map[string]interface{}{
					"prefixes": prefixInterface, // strip 対象プレフィックス一覧を設定する
				},
			},
		},
	}
}

// ApplyMiddleware は Traefik StripPrefix Middleware を作成または更新する
func ApplyMiddleware(ctx context.Context, client dynamic.Interface, ingressRouteID string, namespace string, prefixList []string) error {
	manifest := buildMiddlewareManifest(ingressRouteID, namespace, prefixList) // マニフェストを生成する

	existing, err := client.Resource(traefikMiddlewareGVR).Namespace(namespace).Get(ctx, manifest.GetName(), metav1.GetOptions{}) // 既存の Middleware を取得する
	if err != nil {
		// 存在しない場合は新規作成する
		_, err = client.Resource(traefikMiddlewareGVR).Namespace(namespace).Create(ctx, manifest, metav1.CreateOptions{})
		return err
	}

	// 既存の Middleware を更新する
	manifest.SetResourceVersion(existing.GetResourceVersion()) // 楽観的並行性制御のため ResourceVersion を引き継ぐ
	_, err = client.Resource(traefikMiddlewareGVR).Namespace(namespace).Update(ctx, manifest, metav1.UpdateOptions{})
	return err
}

// DeleteMiddleware は Traefik StripPrefix Middleware を削除する
func DeleteMiddleware(ctx context.Context, client dynamic.Interface, namespace string, ingressRouteID string) error {
	name := middlewareName(ingressRouteID)                                                                    // Middleware 名を生成する
	return client.Resource(traefikMiddlewareGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{}) // Middleware を削除する
}

// ApplyIngressRoute は Traefik IngressRoute を作成または更新する
func ApplyIngressRoute(ctx context.Context, client dynamic.Interface, ingressRouteData models.IngressRoute, namespace string, pathRuleSpecList []PathRuleSpec) error {
	manifest := buildIngressRouteManifest(ingressRouteData, namespace, pathRuleSpecList) // マニフェストを生成する

	existing, err := client.Resource(traefikIngressRouteGVR).Namespace(namespace).Get(ctx, manifest.GetName(), metav1.GetOptions{}) // 既存の IngressRoute を取得する
	if err != nil {
		// 存在しない場合は新規作成する
		_, err = client.Resource(traefikIngressRouteGVR).Namespace(namespace).Create(ctx, manifest, metav1.CreateOptions{})
		return err
	}

	// 既存の IngressRoute を更新する
	manifest.SetResourceVersion(existing.GetResourceVersion()) // 楽観的並行性制御のため ResourceVersion を引き継ぐ
	_, err = client.Resource(traefikIngressRouteGVR).Namespace(namespace).Update(ctx, manifest, metav1.UpdateOptions{})
	return err
}

// DeleteIngressRoute は Traefik IngressRoute を削除する
func DeleteIngressRoute(ctx context.Context, client dynamic.Interface, namespace, name string) error {
	return client.Resource(traefikIngressRouteGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{}) // IngressRoute を削除する
}

// WatchIngressRoutes は全 Namespace の Traefik IngressRoute 変化を監視して DB を自動更新する
func WatchIngressRoutes(ctx context.Context, dynamicClient dynamic.Interface, ingressRouteRepo repository.IngressRouteRepository) {
	for {
		if ctx.Err() != nil { // コンテキストがキャンセルされた場合は終了する
			return
		}

		watcher, err := dynamicClient.Resource(traefikIngressRouteGVR).Namespace("").Watch(ctx, metav1.ListOptions{
			LabelSelector: "launchs-managed=true", // launchs が管理する IngressRoute のみ監視する
		}) // Watch を開始する
		if err != nil {
			logger.PrintErr("WatchIngressRoutes: Watch 開始に失敗しました: " + err.Error()) // エラーをログ出力する
			continue                                                                        // 再試行する
		}

		logger.Println("WatchIngressRoutes: 監視を開始しました") // 監視開始ログを出力する

		ingressRouteWatchLoop(ctx, watcher, ingressRouteRepo) // イベントループを実行する

		logger.Println("WatchIngressRoutes: Watch チャネルが終了しました。再接続します") // 再接続ログを出力する
	}
}

// ingressRouteWatchLoop は IngressRoute Watch イベントチャネルを処理するループ
func ingressRouteWatchLoop(ctx context.Context, watcher watch.Interface, ingressRouteRepo repository.IngressRouteRepository) {
	defer watcher.Stop() // 終了時に Watch を停止する

	for {
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			return
		case event, ok := <-watcher.ResultChan(): // イベントを受信する
			if !ok { // チャネルが閉じられた場合はループを抜ける
				return
			}
			handleIngressRouteEvent(ctx, event, ingressRouteRepo) // イベントを処理する
		}
	}
}

// handleIngressRouteEvent は Traefik IngressRoute の Watch イベントを処理する
func handleIngressRouteEvent(ctx context.Context, event watch.Event, ingressRouteRepo repository.IngressRouteRepository) {
	ingressRouteObj, ok := event.Object.(*unstructured.Unstructured) // イベントオブジェクトを Unstructured にキャストする
	if !ok {                                                           // キャストに失敗した場合はスキップする
		return
	}

	ingressRouteID := ingressRouteObj.GetName() // IngressRoute 名が IngressRoute の ID
	if ingressRouteID == "" {                    // ID が空の場合はスキップする
		return
	}

	if event.Type != watch.Added && event.Type != watch.Modified { // Added/Modified 以外はスキップする
		return
	}

	k8sStatusJSON, err := marshalIngressRouteStatus(ingressRouteObj) // IngressRoute の status を JSON にシリアライズする
	if err != nil {
		logger.PrintErr("WatchIngressRoutes: k8s_status のシリアライズに失敗しました: " + err.Error()) // エラーをログ出力する
		return
	}

	if err := ingressRouteRepo.UpdateStatus(ctx, ingressRouteID, models.IngressRouteStatusActive, k8sStatusJSON); err != nil { // status を active に更新する
		logger.PrintErr("WatchIngressRoutes: status 更新に失敗しました: " + err.Error()) // エラーをログ出力する
		return
	}

	logger.Println("WatchIngressRoutes: status を更新しました: " + ingressRouteID) // 更新ログを出力する
}

// marshalIngressRouteStatus は Unstructured IngressRoute の status フィールドを datatypes.JSON にシリアライズする
func marshalIngressRouteStatus(obj *unstructured.Unstructured) (datatypes.JSON, error) {
	statusRaw := obj.Object["status"] // status フィールドを取得する
	statusBytes, err := json.Marshal(statusRaw) // JSON バイト列に変換する
	if err != nil {
		return nil, err // シリアライズエラーを返す
	}
	return datatypes.JSON(statusBytes), nil // datatypes.JSON に変換して返す
}
