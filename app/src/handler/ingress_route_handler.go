package handler

import (
	"app/logger"
	"app/middlewares"
	"app/service"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// IngressRouteHandler は IngressRoute CRUD の HTTP ハンドラーを提供する
type IngressRouteHandler struct {
	ingressRouteService service.IngressRouteService // IngressRoute サービスのインターフェース
}

// NewIngressRouteHandler は IngressRouteHandler を生成して返す
func NewIngressRouteHandler(ingressRouteService service.IngressRouteService) *IngressRouteHandler {
	return &IngressRouteHandler{
		ingressRouteService: ingressRouteService, // 依存を注入する
	}
}

// GetIngressRoute は GET /projects/:id/ingress-route のハンドラー
func (ingressRouteHandler *IngressRouteHandler) GetIngressRoute(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	projectID := echoCtx.Param("id")                                  // パスパラメータから project ID を取得する

	ingressRouteData, err := ingressRouteHandler.ingressRouteService.GetIngressRoute(echoCtx.Request().Context(), userClaim.UserID, projectID) // サービスを呼び出して ingress_route を取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが見つからない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "ingress_route が見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "GetIngressRoute", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.JSON(http.StatusOK, ingressRouteData) // ingress_route を返す
}

// CreateIngressRoute は POST /projects/:id/ingress-route のハンドラー
func (ingressRouteHandler *IngressRouteHandler) CreateIngressRoute(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	projectID := echoCtx.Param("id")                                  // パスパラメータから project ID を取得する

	var requestBody service.CreateIngressRouteRequest             // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {            // リクエストをバインドする
		logger.PrintHandlerError("IngressRouteHandler", "CreateIngressRoute", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストが不正です"})
	}

	ingressRouteData, err := ingressRouteHandler.ingressRouteService.CreateIngressRoute(echoCtx.Request().Context(), userClaim.UserID, projectID, requestBody) // サービスを呼び出して ingress_route を作成する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "CreateIngressRoute", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.JSON(http.StatusCreated, ingressRouteData) // 作成した ingress_route を返す
}

// UpdateIngressRoute は PUT /projects/:id/ingress-route のハンドラー
func (ingressRouteHandler *IngressRouteHandler) UpdateIngressRoute(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	projectID := echoCtx.Param("id")                                  // パスパラメータから project ID を取得する

	var requestBody service.UpdateIngressRouteRequest             // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {            // リクエストをバインドする
		logger.PrintHandlerError("IngressRouteHandler", "UpdateIngressRoute", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストが不正です"})
	}

	ingressRouteData, err := ingressRouteHandler.ingressRouteService.UpdateIngressRoute(echoCtx.Request().Context(), userClaim.UserID, projectID, requestBody) // サービスを呼び出して pending フィールドを更新する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが見つからない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "ingress_route が見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "UpdateIngressRoute", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.JSON(http.StatusOK, ingressRouteData) // 更新後の ingress_route を返す
}

// DeleteIngressRoute は DELETE /projects/:id/ingress-route のハンドラー
func (ingressRouteHandler *IngressRouteHandler) DeleteIngressRoute(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	projectID := echoCtx.Param("id")                                  // パスパラメータから project ID を取得する

	err := ingressRouteHandler.ingressRouteService.DeleteIngressRoute(echoCtx.Request().Context(), userClaim.UserID, projectID) // サービスを呼び出して ingress_route を削除する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが見つからない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "ingress_route が見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "DeleteIngressRoute", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 204 No Content を返す
}

// ListRoutes は GET /projects/:id/ingress-route/routes のハンドラー
func (ingressRouteHandler *IngressRouteHandler) ListRoutes(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	projectID := echoCtx.Param("id")                                  // パスパラメータから project ID を取得する

	routeList, err := ingressRouteHandler.ingressRouteService.ListRoutes(echoCtx.Request().Context(), userClaim.UserID, projectID) // サービスを呼び出してルートエントリ一覧を取得する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // IngressRoute が見つからない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "ingress_route が見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "ListRoutes", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.JSON(http.StatusOK, routeList) // ルートエントリ一覧を返す
}

// AddRoute は POST /projects/:id/ingress-route/routes のハンドラー
func (ingressRouteHandler *IngressRouteHandler) AddRoute(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	projectID := echoCtx.Param("id")                                  // パスパラメータから project ID を取得する

	var requestBody service.AddRouteRequest                       // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {            // リクエストをバインドする
		logger.PrintHandlerError("IngressRouteHandler", "AddRoute", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストが不正です"})
	}

	routeData, err := ingressRouteHandler.ingressRouteService.AddRoute(echoCtx.Request().Context(), userClaim.UserID, projectID, requestBody) // サービスを呼び出してルートエントリを追加する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		if errors.Is(err, service.ErrDeploymentNotBelongToProject) { // DeploymentID 検証エラーの場合
			return echoCtx.JSON(http.StatusBadRequest, map[string]string{"error": "deployment_id がこのプロジェクトに属していません"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // IngressRoute が見つからない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "ingress_route が見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "AddRoute", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.JSON(http.StatusCreated, routeData) // 作成したルートエントリを返す
}

// UpdateRoute は PUT /projects/:id/ingress-route/routes/:routeId のハンドラー
func (ingressRouteHandler *IngressRouteHandler) UpdateRoute(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	projectID := echoCtx.Param("id")                                  // パスパラメータから project ID を取得する
	routeID := echoCtx.Param("routeId")                               // パスパラメータから route ID を取得する

	var requestBody service.UpdateRouteRequest                    // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {            // リクエストをバインドする
		logger.PrintHandlerError("IngressRouteHandler", "UpdateRoute", echoCtx.Request().URL.Path, http.StatusBadRequest, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストが不正です"})
	}

	routeData, err := ingressRouteHandler.ingressRouteService.UpdateRoute(echoCtx.Request().Context(), userClaim.UserID, projectID, routeID, requestBody) // サービスを呼び出してルートエントリを更新する
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		if errors.Is(err, service.ErrDeploymentNotBelongToProject) { // DeploymentID 検証エラーの場合
			return echoCtx.JSON(http.StatusBadRequest, map[string]string{"error": "deployment_id がこのプロジェクトに属していません"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // ルートエントリが見つからない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "route が見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "UpdateRoute", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.JSON(http.StatusOK, routeData) // 更新後のルートエントリを返す
}

// DeleteRoute は DELETE /projects/:id/ingress-route/routes/:routeId のハンドラー
func (ingressRouteHandler *IngressRouteHandler) DeleteRoute(echoCtx echo.Context) error {
	userClaim := echoCtx.Get("claim").(*middlewares.AccessTokenClaim) // JWT クレームを取得する
	projectID := echoCtx.Param("id")                                  // パスパラメータから project ID を取得する
	routeID := echoCtx.Param("routeId")                               // パスパラメータから route ID を取得する

	err := ingressRouteHandler.ingressRouteService.DeleteRoute(echoCtx.Request().Context(), userClaim.UserID, projectID, routeID) // サービスを呼び出してルートエントリを deleting 状態にする
	if err != nil {
		if errors.Is(err, service.ErrForbidden) { // 所有権エラーの場合
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // ルートエントリが見つからない場合
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "route が見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "DeleteRoute", echoCtx.Request().URL.Path, http.StatusInternalServerError, err) // エラーログを出力する
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 204 No Content を返す
}
