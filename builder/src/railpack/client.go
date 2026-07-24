// Package railpack は Kubernetes 上で BuildKit を使ったコンテナイメージビルドを
// シンプルに実行するためのライブラリです。
//
// 基本的な使い方（GitHubソース）:
//
//	client, err := railpack.New(clientset, railpack.BuildConfig{
//	    GitRepo:        "https://github.com/org/repo",
//	    ImageName:      "my-app",
//	    ImageTag:        "v1.0.0",
//	    Namespace:      "buildkit",
//	})
//
// アーカイブソース（zip/tar.gzアップロード）の場合:
//
//	client, err := railpack.New(clientset, railpack.BuildConfig{
//	    SourceType:       "archive",
//	    ArchiveURL:       "https://file.io/xxxx",
//	    ArchiveEncKeyHex: "...",
//	    ArchiveSHA256Hex: "...",
//	    ImageName:        "my-app",
//	    ImageTag:         "v1.0.0",
//	    Namespace:        "buildkit",
//	})
//
//	jobID, err    := client.Build(ctx)
//	status, err   := client.Status(ctx, jobID)
//	logCh, errCh := client.StreamLogs(ctx, jobID)
//	err           = client.Cancel(ctx, jobID)
package railpack

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
)

// Client はビルドパイプラインの操作インターフェースです。
// New() で作成し、Build / Status / StreamLogs / Cancel を呼び出して使います。
type Client struct {
	clientset kubernetes.Interface // kubernetes.Interface を使うことでテスト時に fake クライアントを注入できる
	config    BuildConfig
}

// New は Client を生成します。
// config には BuildConfig を渡してください。省略可能なフィールドにはデフォルト値が適用されます。
func New(clientset kubernetes.Interface, config BuildConfig) (*Client, error) {
	if clientset == nil { // nil チェックはインターフェース型でも有効
		return nil, fmt.Errorf("clientset は必須です")
	}

	config = applyDefaults(config) // SourceType の自動判定を先に適用する

	if config.SourceType == "archive" { // アーカイブソースの必須項目を検証する
		if config.ArchiveURL == "" {
			return nil, fmt.Errorf("ArchiveURL は必須です")
		}
		if config.ArchiveEncKeyHex == "" {
			return nil, fmt.Errorf("ArchiveEncKeyHex は必須です")
		}
		if config.ArchiveSHA256Hex == "" {
			return nil, fmt.Errorf("ArchiveSHA256Hex は必須です")
		}
	} else { // Gitソースの必須項目を検証する
		if config.GitRepo == "" {
			return nil, fmt.Errorf("GitRepo は必須です")
		}
	}
	if config.RegistryUsername == "" {
		return nil, fmt.Errorf("RegistryUsername は必須です")
	}
	if config.Namespace == "" {
		return nil, fmt.Errorf("Namespace は必須です")
	}
	if config.ImageName == "" {
		return nil, fmt.Errorf("ImageName は必須です")
	}
	if config.ImageTag == "" {
		return nil, fmt.Errorf("ImageTag は必須です")
	}

	return &Client{
		clientset: clientset,
		config:    config,
	}, nil
}

// Build はビルドジョブを Kubernetes 上に作成し、jobID を返します。
// jobID を使って Status / StreamLogs / Cancel を呼び出してください。
// この関数はジョブを起動するだけで、完了を待ちません。
func (client *Client) Build(ctx context.Context) (jobID string, err error) {
	jobID, err = createJob(ctx, client.clientset, client.config.Namespace, client.config)
	if err != nil {
		return "", fmt.Errorf("ジョブの作成に失敗しました: %w", err)
	}
	return jobID, nil
}

// Status は指定した jobID の現在の状態を返します。
//
// 戻り値は以下のいずれかです:
//   - StatusInit     — Job作成済み、Pod起動待ち
//   - StatusRunning  — ビルド実行中
//   - StatusComplete — ビルド成功
//   - StatusFailed   — ビルド失敗
func (client *Client) Status(ctx context.Context, jobID string) (BuildStatus, error) {
	status, err := getJobStatus(ctx, client.clientset, client.config.Namespace, jobID)
	if err != nil {
		return StatusFailed, fmt.Errorf("ステータスの取得に失敗しました: %w", err)
	}
	return status, nil
}

// Cancel は指定した jobID のビルドジョブを強制停止します。
func (client *Client) Cancel(ctx context.Context, jobID string) error {
	if err := deleteJob(ctx, client.clientset, client.config.Namespace, jobID); err != nil {
		return fmt.Errorf("ジョブの停止に失敗しました: %w", err)
	}
	return nil
}

// Wait はビルドが完了（Complete または Failed）するまでブロックします。
// ポーリング間隔は 10 秒です。
// タイムアウトは BuildConfig.Timeout で設定します。
func (client *Client) Wait(ctx context.Context, jobID string) (BuildStatus, error) {
	deadline := time.Now().Add(client.config.Timeout)

	for {
		if time.Now().After(deadline) {
			return StatusFailed, fmt.Errorf("タイムアウト: %s 経過しました", client.config.Timeout)
		}

		status, err := client.Status(ctx, jobID)
		if err != nil {
			return StatusFailed, err
		}

		switch status {
		case StatusComplete, StatusFailed:
			return status, nil
		}

		select {
		case <-ctx.Done():
			return StatusFailed, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}
