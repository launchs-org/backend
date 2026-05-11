package harbor

import (
	"context"
	"fmt"
	"net/http"
)

// RobotCredential は作成されたプロジェクトスコープ robot の認証情報です。
// secret は作成時のレスポンスにのみ含まれ、以降は取得できません。
type RobotCredential struct {
	ID     int64
	Name   string // Harbor が付与する "robot$<project>+<name>" 形式のフルネーム
	Secret string
}

// CreateProjectRobot はプロジェクトスコープの robot アカウントを作成します。
// name は robot の短縮名（プロジェクト内で一意）です。
// Harbor は "robot$<project>+<name>" 形式のフルネームを自動付与します。
// duration に -1 を渡すと無期限になります。
func (c *Client) CreateProjectRobot(ctx context.Context, projectName, name string, durationDays int64) (*RobotCredential, error) {
	body := map[string]any{
		"name":        name,
		"description": fmt.Sprintf("build robot for project %s", projectName),
		"duration":    durationDays,
		"level":       "project",
		"permissions": []map[string]any{
			{
				"kind":      "project",
				"namespace": projectName,
				"access": []map[string]string{
					{"resource": "repository", "action": "push"},
					{"resource": "repository", "action": "pull"},
					{"resource": "artifact",   "action": "read"},
					{"resource": "tag",        "action": "create"},
					{"resource": "tag",        "action": "delete"},
				},
			},
		},
	}

	resp, err := c.doJSON(ctx, "POST", "/api/v2.0/robots", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, http.StatusCreated); err != nil {
		return nil, fmt.Errorf("robot 作成失敗: %w", err)
	}

	var created struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Secret string `json:"secret"`
	}
	if err := decodeJSON(resp, &created); err != nil {
		return nil, err
	}

	return &RobotCredential{
		ID:     created.ID,
		Name:   created.Name,
		Secret: created.Secret,
	}, nil
}

// DeleteRobot は robot ID を指定して robot アカウントを削除します。
func (c *Client) DeleteRobot(ctx context.Context, robotID int64) error {
	resp, err := c.doJSON(ctx, "DELETE", fmt.Sprintf("/api/v2.0/robots/%d", robotID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp, http.StatusOK, http.StatusNoContent)
}
