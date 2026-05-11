package harbor

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// CleanupExpiredUsers は コメントフィールドに埋め込んだ有効期限を見て
// 期限切れの CI ユーザーを一括削除します。
//
// CreateEphemeralUser が生成するコメント形式:
//
//	"ephemeral CI user | project=<name> role=<role> expires=<RFC3339>"
func (c *Client) CleanupExpiredUsers(ctx context.Context) (deleted int, err error) {
	users, err := c.listCIUsers(ctx)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	for _, u := range users {
		exp, parseErr := parseExpiry(u.Comment)
		if parseErr != nil {
			log.Printf("⚠️  ユーザー '%s' の有効期限をパースできません: %v", u.Username, parseErr)
			continue
		}
		if now.After(exp) {
			if delErr := c.DeleteUser(ctx, u.Username); delErr != nil {
				log.Printf("⚠️  ユーザー '%s' の削除失敗: %v", u.Username, delErr)
				continue
			}
			log.Printf("🗑️  期限切れユーザー '%s' を削除しました (期限: %s)", u.Username, exp.Format(time.RFC3339))
			deleted++
		}
	}
	return deleted, nil
}

// ── 内部ヘルパー ─────────────────────────────────────────────────────────────

type userDetail struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Comment  string `json:"comment"`
}

func (c *Client) listCIUsers(ctx context.Context) ([]userDetail, error) {
	resp, err := c.doJSON(ctx, "GET", "/api/v2.0/users?page_size=100", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, 200); err != nil {
		return nil, err
	}

	var all []userDetail
	if err := decodeJSON(resp, &all); err != nil {
		return nil, err
	}

	var ciUsers []userDetail
	for _, u := range all {
		if strings.HasPrefix(u.Comment, "ephemeral CI user") {
			ciUsers = append(ciUsers, u)
		}
	}
	return ciUsers, nil
}

func parseExpiry(comment string) (time.Time, error) {
	const marker = "expires="
	idx := strings.Index(comment, marker)
	if idx == -1 {
		return time.Time{}, fmt.Errorf("'expires=' フィールドが見つかりません")
	}
	raw := strings.TrimSpace(comment[idx+len(marker):])
	// スペースや末尾の余分な文字を除去
	if sp := strings.IndexAny(raw, " \t\n"); sp != -1 {
		raw = raw[:sp]
	}
	return time.Parse(time.RFC3339, raw)
}
