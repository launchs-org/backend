package k8s

import (
	"app/logger"
	"app/models"
	"app/repository"
	"context"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

var invalidNamespaceChars = regexp.MustCompile(`[^a-z0-9-]`) // namespace に使用できない文字にマッチする正規表現

// ToNamespaceName はプロジェクト名を k8s namespace 名として有効な文字列に変換する
// 大文字を小文字に変換し、英小文字・数字・ハイフン以外の文字をハイフンに置換する
func ToNamespaceName(name string) string {
	lower := strings.ToLower(name)                             // 大文字を小文字に変換する
	replaced := invalidNamespaceChars.ReplaceAllString(lower, "-") // 使用不可文字をハイフンに置換する
	trimmed := strings.Trim(replaced, "-")                     // 先頭・末尾のハイフンを除去する
	return trimmed
}

// CreateNamespace は指定した名前の k8s namespace を作成し、NetworkPolicy を適用する
func CreateNamespace(ctx context.Context, client kubernetes.Interface, name string) error {
	namespaceObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, // namespace 名を設定する
			Labels: map[string]string{
				"launchs.org/managed": "true",        // このサービスが管理する namespace であることを示すラベル
				"kubernetes.io/metadata.name": name,  // NetworkPolicy の namespaceSelector で参照するためのラベル
			},
		},
	}
	_, err := client.CoreV1().Namespaces().Create(ctx, namespaceObj, metav1.CreateOptions{}) // namespace を作成する
	if err != nil {
		return err // namespace 作成に失敗した場合はエラーを返す
	}

	return applyNamespaceNetworkPolicy(ctx, client, name) // NetworkPolicy を適用する
}

// applyNamespaceNetworkPolicy は traefik・cloudflared・自身の namespace からのみ Ingress を許可する NetworkPolicy を適用する
func applyNamespaceNetworkPolicy(ctx context.Context, client kubernetes.Interface, namespaceName string) error {
	egressAllowAll := networkingv1.NetworkPolicyEgressRule{} // Egress は全て許可する
	networkPolicyObj := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-traefik-cloudflared-local", // NetworkPolicy 名を設定する
			Namespace: namespaceName,                     // 対象 namespace を設定する
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{}, // namespace 内の全 Pod を対象にする
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress, // Ingress ポリシーを有効にする
				networkingv1.PolicyTypeEgress,  // Egress ポリシーを有効にする
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{ // traefik namespace からのアクセスを許可する
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "traefik",
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{ // cloudflared namespace からのアクセスを許可する
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "cloudflared",
								},
							},
						},
						{
							NamespaceSelector: &metav1.LabelSelector{ // 自身の namespace 内からのアクセスを許可する
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": namespaceName,
								},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{egressAllowAll}, // Egress は全て許可する
		},
	}
	_, err := client.NetworkingV1().NetworkPolicies(namespaceName).Create(ctx, networkPolicyObj, metav1.CreateOptions{}) // NetworkPolicy を作成する
	return err
}

// DeleteNamespace は指定した名前の k8s namespace を削除する
func DeleteNamespace(ctx context.Context, client kubernetes.Interface, name string) error {
	return client.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{}) // namespace を削除する
}

// WatchNamespaces は launchs.org/managed=true ラベルを持つ k8s Namespace の削除イベントを監視して DB の Project レコードを削除する
func WatchNamespaces(ctx context.Context, k8sClient kubernetes.Interface, projectRepo repository.ProjectRepository) {
	for {
		if ctx.Err() != nil { // コンテキストがキャンセルされた場合は終了する
			return
		}

		watcher, err := k8sClient.CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{
			LabelSelector: "launchs.org/managed=true", // このサービスが管理する Namespace のみ監視する
		}) // Watch を開始する
		if err != nil {
			logger.PrintErr("WatchNamespaces: Watch 開始に失敗しました: " + err.Error()) // エラーをログ出力する
			continue                                                                     // 再試行する
		}

		logger.Println("WatchNamespaces: 監視を開始しました") // 監視開始ログを出力する

		namespaceWatchLoop(ctx, watcher, projectRepo) // イベントループを実行する

		logger.Println("WatchNamespaces: Watch チャネルが終了しました。再接続します") // 再接続ログを出力する
	}
}

// namespaceWatchLoop は k8s Namespace Watch イベントチャネルを処理するループ
func namespaceWatchLoop(ctx context.Context, watcher watch.Interface, projectRepo repository.ProjectRepository) {
	for {
		select {
		case <-ctx.Done(): // コンテキストがキャンセルされた場合は終了する
			watcher.Stop() // Watch を停止する
			return
		case event, ok := <-watcher.ResultChan(): // イベントチャネルから受信する
			if !ok { // チャネルが閉じられた場合は再接続のためにループを抜ける
				return
			}
			handleNamespaceEvent(ctx, event, projectRepo) // イベントを処理する
		}
	}
}

// handleNamespaceEvent は Namespace の Watch イベントを処理する
func handleNamespaceEvent(ctx context.Context, event watch.Event, projectRepo repository.ProjectRepository) {
	if event.Type != watch.Deleted { // Deleted イベント以外はスキップする
		return
	}

	namespaceObj, ok := event.Object.(*corev1.Namespace) // イベントオブジェクトを Namespace にキャストする
	if !ok {                                               // キャストに失敗した場合はスキップする
		return
	}

	namespaceName := namespaceObj.Name // Namespace 名を取得する

	projectData, err := projectRepo.FindByNamespace(ctx, namespaceName) // Namespace 名に対応する Project を取得する
	if err != nil {
		logger.PrintErr("WatchNamespaces: Project の取得に失敗しました (namespace=" + namespaceName + "): " + err.Error()) // エラーをログ出力する
		return
	}

	if projectData.Status != models.ProjectStatusDeleting { // status が deleting でない場合は意図しない削除とみなす
		logger.PrintErr("WatchNamespaces: Project の status が deleting ではないため削除をスキップします (namespace=" + namespaceName + ", status=" + string(projectData.Status) + ")") // 警告ログを出力する
		return
	}

	if err := projectRepo.DeleteNoTx(ctx, projectData); err != nil { // Project レコードを削除する
		logger.PrintErr("WatchNamespaces: Project の削除に失敗しました (namespace=" + namespaceName + "): " + err.Error()) // エラーをログ出力する
		return
	}

	logger.Println("WatchNamespaces: Project を削除しました (namespace=" + namespaceName + ", projectID=" + projectData.ID + ")") // 削除ログを出力する
}
