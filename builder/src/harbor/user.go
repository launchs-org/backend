package harbor

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Role は Harbor のプロジェクトメンバーロールを表します。
type Role int

const (
	RoleProjectAdmin  Role = 1
	RoleDeveloper     Role = 2
	RoleGuest         Role = 3
	RoleMaintainer    Role = 4
	RoleLimitedGuest  Role = 5
)

// EphemeralUserRequest は短命 CI ユーザー作成のリクエストです。
type EphemeralUserRequest struct {
	ProjectName string
	Role        Role
	TTL         time.Duration
}

// EphemeralUser は作成された短命ユーザーの情報です。
type EphemeralUser struct {
	Username  string
	Password  string
	ExpiresAt time.Time
}

// CreateProjectUser はプロジェクト専用の永続ユーザーを作成し、指定プロジェクトにバインドします。
// ユーザー名は "build-<projectID>" 形式で固定されます。
// projectID はアプリ側の識別子で、Harbor プロジェクト名 (harborProject) と別に指定できます。
func (c *Client) CreateProjectUser(ctx context.Context, projectID, harborProject string, role Role) (*EphemeralUser, error) {
	username := fmt.Sprintf("build-%s", projectID)
	password := generatePassword()
	comment := fmt.Sprintf("build user | project=%s", projectID)

	createBody := map[string]any{
		"username": username,
		"password": password,
		"email":    username + "@build.local",
		"realname": username,
		"comment":  comment,
	}
	resp, err := c.doJSON(ctx, "POST", "/api/v2.0/users", createBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, http.StatusCreated); err != nil {
		return nil, fmt.Errorf("ユーザー作成失敗: %w", err)
	}

	userID, err := c.getUserID(ctx, username)
	if err != nil {
		return nil, err
	}

	memberBody := map[string]any{
		"role_id": int(role),
		"member_user": map[string]any{
			"user_id": userID,
		},
	}
	mresp, err := c.doJSON(ctx, "POST",
		fmt.Sprintf("/api/v2.0/projects/%s/members", harborProject), memberBody)
	if err != nil {
		return nil, err
	}
	defer mresp.Body.Close()
	if err := checkStatus(mresp, http.StatusCreated); err != nil {
		return nil, fmt.Errorf("メンバー追加失敗: %w", err)
	}

	return &EphemeralUser{
		Username:  username,
		Password:  password,
		ExpiresAt: time.Time{}, // 永続ユーザーは期限なし
	}, nil
}

// CreateEphemeralUser は CI 用の短命ユーザーを作成し、指定プロジェクトにバインドします。
// ユーザー名は "ci-<project>-<unix>" 形式で一意になります。
func (c *Client) CreateEphemeralUser(ctx context.Context, req EphemeralUserRequest) (*EphemeralUser, error) {
	expiresAt := time.Now().Add(req.TTL)
	username := fmt.Sprintf("ci-%s-%d", req.ProjectName, time.Now().UnixNano())
	password := generatePassword()
	comment := fmt.Sprintf("ephemeral CI user | project=%s role=%d expires=%s",
		req.ProjectName, req.Role, expiresAt.UTC().Format(time.RFC3339))

	// ユーザー作成
	createBody := map[string]any{
		"username": username,
		"password": password,
		"email":    username + "@ci.local",
		"realname": username,
		"comment":  comment,
	}
	resp, err := c.doJSON(ctx, "POST", "/api/v2.0/users", createBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, http.StatusCreated); err != nil {
		return nil, fmt.Errorf("ユーザー作成失敗: %w", err)
	}

	// ユーザー ID を取得
	userID, err := c.getUserID(ctx, username)
	if err != nil {
		return nil, err
	}

	// プロジェクトメンバーとして追加
	memberBody := map[string]any{
		"role_id": int(req.Role),
		"member_user": map[string]any{
			"user_id": userID,
		},
	}
	mresp, err := c.doJSON(ctx, "POST",
		fmt.Sprintf("/api/v2.0/projects/%s/members", req.ProjectName), memberBody)
	if err != nil {
		return nil, err
	}
	defer mresp.Body.Close()
	if err := checkStatus(mresp, http.StatusCreated); err != nil {
		return nil, fmt.Errorf("メンバー追加失敗: %w", err)
	}

	return &EphemeralUser{
		Username:  username,
		Password:  password,
		ExpiresAt: expiresAt,
	}, nil
}

// DeleteUser はユーザーを削除します。
func (c *Client) DeleteUser(ctx context.Context, username string) error {
	userID, err := c.getUserID(ctx, username)
	if err != nil {
		return err
	}
	resp, err := c.doJSON(ctx, "DELETE", fmt.Sprintf("/api/v2.0/users/%d", userID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkStatus(resp, http.StatusNoContent)
}

func (c *Client) getUserID(ctx context.Context, username string) (int, error) {
	resp, err := c.doJSON(ctx, "GET", "/api/v2.0/users?username="+username, nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp, http.StatusOK); err != nil {
		return 0, err
	}
	var users []struct {
		UserID   int    `json:"user_id"`
		Username string `json:"username"`
	}
	if err := decodeJSON(resp, &users); err != nil {
		return 0, err
	}
	for _, u := range users {
		if u.Username == username {
			return u.UserID, nil
		}
	}
	return 0, fmt.Errorf("ユーザー '%s' が見つかりません", username)
}
