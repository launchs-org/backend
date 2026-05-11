package harbor

import (
	"context"
	"fmt"
	"log"
	"net/http"
)

// CreateProject は Harbor にプロジェクト（リポジトリ名前空間）を作成します。
// すでに存在する場合は 409 を無視して成功扱いにします。
func (c *Client) CreateProject(ctx context.Context, name string, public bool) error {
	body := map[string]any{
		"project_name": name,
		"public":       public,
		"metadata":     map[string]string{"public": boolStr(public)},
	}
	resp, err := c.doJSON(ctx, "POST", "/api/v2.0/projects", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// ログを出す
	log.Printf("[harbor] プロジェクト '%s' を作成しました", name)

	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	return checkStatus(resp, http.StatusCreated)
}

// DeleteProject はプロジェクトを削除します。
func (c *Client) DeleteProject(ctx context.Context, name string) error {
	resp, err := c.doJSON(ctx, "DELETE",
		fmt.Sprintf("/api/v2.0/projects/%s?is_resource_name=true", name), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp, http.StatusOK, http.StatusNoContent)
}


func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
