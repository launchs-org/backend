package k8s

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewClient_正常系 は kubeconfig が存在する場合にクライアントが生成されることを確認する
func TestNewClient_正常系(t *testing.T) {
	// テスト用の最小限 kubeconfig を一時ファイルとして作成する
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: dummy-token
`
	tempDir := t.TempDir()                                                       // テスト用の一時ディレクトリを作成する
	kubeDir := filepath.Join(tempDir, ".kube")                                   // .kube ディレクトリのパスを組み立てる
	err := os.MkdirAll(kubeDir, 0755)                                            // .kube ディレクトリを作成する
	if err != nil {
		t.Fatalf(".kube ディレクトリの作成に失敗しました: %v", err) // ディレクトリ作成失敗時はテストを中断する
	}
	kubeconfigPath := filepath.Join(kubeDir, "config")                           // kubeconfig パスを組み立てる
	err = os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)          // 一時 kubeconfig ファイルを書き込む
	if err != nil {
		t.Fatalf("kubeconfig ファイルの作成に失敗しました: %v", err) // ファイル作成失敗時はテストを中断する
	}

	originalHome := os.Getenv("HOME")                       // 元の HOME 環境変数を保存する
	defer os.Setenv("HOME", originalHome)                   // テスト終了後に HOME を元に戻す
	os.Setenv("HOME", tempDir)                              // HOME を一時ディレクトリに変更して kubeconfig を差し替える

	clientset, err := NewClient() // クライアントを生成する
	if err != nil {
		t.Fatalf("NewClient() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}
	if clientset == nil {
		t.Fatal("NewClient() が nil を返しました") // nil が返った場合はテスト失敗とする
	}
}

// TestNewClient_kubeconfigなし は kubeconfig が存在しない場合にエラーが返ることを確認する
func TestNewClient_kubeconfigなし(t *testing.T) {
	emptyDir := t.TempDir()                        // kubeconfig が存在しない一時ディレクトリを作成する
	originalHome := os.Getenv("HOME")              // 元の HOME 環境変数を保存する
	defer os.Setenv("HOME", originalHome)          // テスト終了後に HOME を元に戻す
	os.Setenv("HOME", emptyDir)                    // HOME を空ディレクトリに変更して kubeconfig が存在しない状態を作る

	_, err := NewClient() // kubeconfig なしでクライアント生成を試みる
	if err == nil {
		t.Fatal("kubeconfig が存在しない場合に NewClient() はエラーを返すべきです") // エラーが返らない場合はテスト失敗とする
	}
}

// TestNewDynamicClient_正常系 は kubeconfig が存在する場合に dynamic クライアントが生成されることを確認する
func TestNewDynamicClient_正常系(t *testing.T) {
	// テスト用の最小限 kubeconfig を一時ファイルとして作成する
	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: dummy-token
`
	tempDir := t.TempDir()                                                       // テスト用の一時ディレクトリを作成する
	kubeDir := filepath.Join(tempDir, ".kube")                                   // .kube ディレクトリのパスを組み立てる
	err := os.MkdirAll(kubeDir, 0755)                                            // .kube ディレクトリを作成する
	if err != nil {
		t.Fatalf(".kube ディレクトリの作成に失敗しました: %v", err) // ディレクトリ作成失敗時はテストを中断する
	}
	kubeconfigPath := filepath.Join(kubeDir, "config")                           // kubeconfig パスを組み立てる
	err = os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0600)          // 一時 kubeconfig ファイルを書き込む
	if err != nil {
		t.Fatalf("kubeconfig ファイルの作成に失敗しました: %v", err) // ファイル作成失敗時はテストを中断する
	}

	originalHome := os.Getenv("HOME")                       // 元の HOME 環境変数を保存する
	defer os.Setenv("HOME", originalHome)                   // テスト終了後に HOME を元に戻す
	os.Setenv("HOME", tempDir)                              // HOME を一時ディレクトリに変更して kubeconfig を差し替える

	dynamicClient, err := NewDynamicClient() // dynamic クライアントを生成する
	if err != nil {
		t.Fatalf("NewDynamicClient() がエラーを返しました: %v", err) // エラーが返った場合はテスト失敗とする
	}
	if dynamicClient == nil {
		t.Fatal("NewDynamicClient() が nil を返しました") // nil が返った場合はテスト失敗とする
	}
}

// TestNewDynamicClient_kubeconfigなし は kubeconfig が存在しない場合にエラーが返ることを確認する
func TestNewDynamicClient_kubeconfigなし(t *testing.T) {
	emptyDir := t.TempDir()                        // kubeconfig が存在しない一時ディレクトリを作成する
	originalHome := os.Getenv("HOME")              // 元の HOME 環境変数を保存する
	defer os.Setenv("HOME", originalHome)          // テスト終了後に HOME を元に戻す
	os.Setenv("HOME", emptyDir)                    // HOME を空ディレクトリに変更して kubeconfig が存在しない状態を作る

	_, err := NewDynamicClient() // kubeconfig なしで dynamic クライアント生成を試みる
	if err == nil {
		t.Fatal("kubeconfig が存在しない場合に NewDynamicClient() はエラーを返すべきです") // エラーが返らない場合はテスト失敗とする
	}
}
