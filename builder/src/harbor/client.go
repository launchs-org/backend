// Package harbor provides a client for the Harbor v2 REST API.
// Supported operations:
//   - Project (repository namespace) create / delete  → project.go
//   - Role definitions                                → role.go
//   - Ephemeral CI user create / delete               → user.go
//   - Expired user cleanup                            → cleanup.go
//   - Password generation                             → password.go
package harbor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ── Config / Client ──────────────────────────────────────────────────────────

// Config は Harbor クライアントの設定です。
type Config struct {
	BaseURL    string       // 例: https://harbor.example.com
	Username   string       // 管理者ユーザー名
	Password   string       // 管理者パスワード
	HTTPClient *http.Client // nil の場合はデフォルトクライアントを使用
}

// Client は Harbor API クライアントです。
type Client struct {
	cfg  Config
	http *http.Client
}

// NewClient は新しい Harbor クライアントを作成します。
func NewClient(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Client{cfg: cfg, http: hc}
}

// ── HTTP ヘルパー ─────────────────────────────────────────────────────────────

func (c *Client) doJSON(
	ctx context.Context, method, path string, body any,
) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("リクエストボディの JSON 化失敗: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("リクエスト生成失敗: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.cfg.Username, c.cfg.Password)

	return c.http.Do(req)
}

func checkStatus(resp *http.Response, allowed ...int) error {
	for _, code := range allowed {
		if resp.StatusCode == code {
			return nil
		}
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("予期しないステータス %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}
