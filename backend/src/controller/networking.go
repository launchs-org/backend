package controller

import (
	"backend/service"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

// UpdateServiceRequest はサービス更新のリクエストボディです
type UpdateServiceRequest struct {
	Type     string                `json:"type"`      // Serviceタイプ
	Ports    []service.ServicePort `json:"ports"`     // ポート設定
	IsActive *bool                 `json:"is_active"` // 有効フラグ
}

// UpdateService はサービス設定を更新するハンドラーです
func UpdateService(ctx *echo.Context) error {
	// リクエストボディをバインド
	var req UpdateServiceRequest
	if err := (*ctx).Bind(&req); err != nil {
		// バインド失敗時は400エラー
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{
			"code":    "BAD_REQUEST",
			"message": "リクエストパラメータが不正です",
		})
	}

	// パスパラメータからコンテナIDを取得
	containerID := (*ctx).Param("id")
	// ユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		// 取得失敗時は401エラー
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// デフォルト値の設定
	svcType := "LoadBalancer"

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	// サービス層の更新関数を呼び出す
	res, err := service.UpdateService((*ctx).Request().Context(), containerID, userID, svcType, req.Ports, isActive)
	if err != nil {
		// エラー内容に応じてステータスコードを振り分け
		if err == service.ErrContainerNotFound {
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "コンテナが見つかりません",
			})
		}
		if err == service.ErrForbidden {
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		}
		// その他のエラーは500
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		})
	}

	// 成功時は200 OK
	return (*ctx).JSON(http.StatusOK, res)
}

// CreateIngressRequest はIngress作成のリクエストボディです
type CreateIngressRequest struct {
	HttpPort            int    `json:"http_port"`              // 公開するポート
	CustomDomain        string `json:"custom_domain"`         // カスタムドメイン
	CustomDomainEnabled bool   `json:"custom_domain_enabled"` // カスタムドメイン有効フラグ
}

// CreateIngress はIngressを作成するハンドラーです
func CreateIngress(ctx *echo.Context) error {
	// リクエストボディをバインド
	var req CreateIngressRequest
	if err := (*ctx).Bind(&req); err != nil {
		// バインド失敗時は400エラー
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{
			"code":    "BAD_REQUEST",
			"message": "リクエストパラメータが不正です",
		})
	}

	// パスパラメータからコンテナIDを取得
	containerID := (*ctx).Param("id")
	// ユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		// 取得失敗時は401エラー
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// サービス層の作成関数を呼び出す
	res, err := service.CreateIngress((*ctx).Request().Context(), containerID, userID, req.CustomDomain, req.CustomDomainEnabled, req.HttpPort)
	if err != nil {
		// すでに存在する場合のエラーハンドリング
		if strings.Contains(err.Error(), "already exists") {
			return (*ctx).JSON(http.StatusConflict, map[string]string{
				"code":    "CONFLICT",
				"message": "Ingressは既に存在します",
			})
		}
		// 権限エラー
		if err == service.ErrForbidden {
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		}
		if err == service.ErrIngressNotAllowed {
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "INGRESS_NOT_ALLOWED",
				"message": "このテンプレートでは外部公開は許可されていません",
			})
		}
		// その他のエラーは500
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		})
	}

	// 成功時は201 Created
	return (*ctx).JSON(http.StatusCreated, res)
}

// UpdateIngressRequest はIngress更新のリクエストボディです
type UpdateIngressRequest struct {
	HttpPort            int    `json:"http_port"`              // 公開するポート
	CustomDomain        string `json:"custom_domain"`         // カスタムドメイン
	CustomDomainEnabled bool   `json:"custom_domain_enabled"` // カスタムドメイン有効フラグ
}

// UpdateIngress はIngressを更新するハンドラーです
func UpdateIngress(ctx *echo.Context) error {
	// リクエストボディをバインド
	var req UpdateIngressRequest
	if err := (*ctx).Bind(&req); err != nil {
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{
			"code":    "BAD_REQUEST",
			"message": "リクエストパラメータが不正です",
		})
	}

	// パスパラメータからコンテナIDを取得
	containerID := (*ctx).Param("id")
	// ユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// サービス層の更新関数を呼び出す
	res, err := service.UpdateIngress((*ctx).Request().Context(), containerID, userID, req.CustomDomain, req.CustomDomainEnabled, req.HttpPort)
	if err != nil {
		if err == service.ErrForbidden {
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		}
		if strings.Contains(err.Error(), "not found") {
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "Ingressが見つかりません",
			})
		}
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		})
	}

	// 成功時は200 OK
	return (*ctx).JSON(http.StatusOK, res)
}

// DeleteIngress はIngressを削除するハンドラーです
func DeleteIngress(ctx *echo.Context) error {
	// パスパラメータからコンテナIDを取得
	containerID := (*ctx).Param("id")
	// ユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	if !ok {
		// 取得失敗時は401エラー
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED",
			"message": "認証に失敗しました",
		})
	}

	// サービス層の削除関数を呼び出す
	res, err := service.DeleteIngressRoute((*ctx).Request().Context(), containerID, userID)
	if err != nil {
		// 権限エラー
		if err == service.ErrForbidden {
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN",
				"message": "アクセス権限がありません",
			})
		}
		// 見つからない場合
		if strings.Contains(err.Error(), "not found") {
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND",
				"message": "Ingressが見つかりません",
			})
		}
		// その他のエラーは500
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
			"code":    "INTERNAL_ERROR",
			"message": err.Error(),
		})
	}

	// 成功時は200 OK
	return (*ctx).JSON(http.StatusOK, res)
}
