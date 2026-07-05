package handler

import (
	"handler/logger"
	"handler/service"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// IngressRouteHandler は IngressRoute・PathRule CRUD の HTTP ハンドラーを提供する
type IngressRouteHandler struct {
	ingressRouteService service.IngressRouteService  // IngressRoute サービスのインターフェース
	applyService        service.ApplyServiceInterface // apply サービスのインターフェース
}

// NewIngressRouteHandler は IngressRouteHandler を生成して返す
func NewIngressRouteHandler(ingressRouteService service.IngressRouteService, applyService service.ApplyServiceInterface) *IngressRouteHandler {
	return &IngressRouteHandler{
		ingressRouteService: ingressRouteService, // 依存を注入する
		applyService:        applyService,        // apply サービスを注入する
	}
}

// ListIngressRoutes は GET /projects/:id/ingress-routes のハンドラー
func (ingressRouteHandler *IngressRouteHandler) ListIngressRoutes(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	projectID := echoCtx.Param("id")         // パスパラメータから project ID を取得する

	ingressRouteList, err := ingressRouteHandler.ingressRouteService.ListIngressRoutes(echoCtx.Request().Context(), userID, projectID) // サービスを呼び出して ingress_route 一覧を取得する
	if err != nil {                                                                                                                      // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("IngressRouteHandler", "ListIngressRoutes", echoCtx.Request().URL.Path, http.StatusForbidden, err)
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "アクセスが禁止されています"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "ListIngressRoutes", echoCtx.Request().URL.Path, http.StatusInternalServerError, err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.JSON(http.StatusOK, ingressRouteList) // ingress_route 一覧を返す
}

// CreateIngressRouteRequest は POST /projects/:id/ingress-route のリクエスト構造体
type CreateIngressRouteRequest struct {
	Name string `json:"name"` // IngressRoute の名前（省略時はプロジェクト名から自動生成）
}

// UpdateIngressRouteNameRequest は PATCH /ingress-routes/:id/name のリクエスト構造体
type UpdateIngressRouteNameRequest struct {
	Name string `json:"name"` // 変更後の名前
}

// CreateIngressRoute は POST /projects/:id/ingress-route のハンドラー
func (ingressRouteHandler *IngressRouteHandler) CreateIngressRoute(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	projectID := echoCtx.Param("id")         // パスパラメータから project ID を取得する

	var requestBody CreateIngressRouteRequest                  // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {        // リクエストをバインドする
		logger.PrintHandlerError("IngressRouteHandler", "CreateIngressRoute", echoCtx.Request().URL.Path, http.StatusBadRequest, err)
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストが不正です"})
	}

	ingressRouteData, err := ingressRouteHandler.ingressRouteService.CreateIngressRoute(echoCtx.Request().Context(), userID, projectID, requestBody.Name) // サービスを呼び出して ingress_route を作成する
	if err != nil {                                                                                                                                          // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("IngressRouteHandler", "CreateIngressRoute", echoCtx.Request().URL.Path, http.StatusForbidden, err)
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "アクセスが禁止されています"})
		}
		if errors.Is(err, service.ErrInvalidIngressRouteName) { // 名前が不正な場合は 400 を返す
			logger.PrintHandlerError("IngressRouteHandler", "CreateIngressRoute", echoCtx.Request().URL.Path, http.StatusBadRequest, err)
			return echoCtx.JSON(http.StatusBadRequest, map[string]string{"error": "名前は英小文字・数字・ハイフンのみ使用可能で、最大20文字です"})
		}
		if errors.Is(err, service.ErrDuplicateIngressRouteName) { // 同名が既に存在する場合は 409 を返す
			logger.PrintHandlerError("IngressRouteHandler", "CreateIngressRoute", echoCtx.Request().URL.Path, http.StatusConflict, err)
			return echoCtx.JSON(http.StatusConflict, map[string]string{"error": "同じ名前の IngressRoute がこのプロジェクトに既に存在します"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "CreateIngressRoute", echoCtx.Request().URL.Path, http.StatusInternalServerError, err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.JSON(http.StatusCreated, ingressRouteData) // 作成した ingress_route を返す
}

// UpdateIngressRouteName は PATCH /ingress-routes/:id/name のハンドラー
func (ingressRouteHandler *IngressRouteHandler) UpdateIngressRouteName(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string)      // ミドルウェアがセットした UserID を取得する
	ingressRouteID := echoCtx.Param("id")         // パスパラメータから ingress_route ID を取得する

	var requestBody UpdateIngressRouteNameRequest              // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {        // リクエストをバインドする
		logger.PrintHandlerError("IngressRouteHandler", "UpdateIngressRouteName", echoCtx.Request().URL.Path, http.StatusBadRequest, err)
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストが不正です"})
	}

	if err := ingressRouteHandler.ingressRouteService.UpdateIngressRouteName(echoCtx.Request().Context(), userID, ingressRouteID, requestBody.Name); err != nil { // サービスを呼び出して名前を変更する
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("IngressRouteHandler", "UpdateIngressRouteName", echoCtx.Request().URL.Path, http.StatusForbidden, err)
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "アクセスが禁止されています"})
		}
		if errors.Is(err, service.ErrInvalidIngressRouteName) { // 名前が不正な場合は 400 を返す
			logger.PrintHandlerError("IngressRouteHandler", "UpdateIngressRouteName", echoCtx.Request().URL.Path, http.StatusBadRequest, err)
			return echoCtx.JSON(http.StatusBadRequest, map[string]string{"error": "名前は英小文字・数字・ハイフンのみ使用可能で、最大20文字です"})
		}
		if errors.Is(err, service.ErrDuplicateIngressRouteName) { // 同名が既に存在する場合は 409 を返す
			logger.PrintHandlerError("IngressRouteHandler", "UpdateIngressRouteName", echoCtx.Request().URL.Path, http.StatusConflict, err)
			return echoCtx.JSON(http.StatusConflict, map[string]string{"error": "同じ名前の IngressRoute がこのプロジェクトに既に存在します"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("IngressRouteHandler", "UpdateIngressRouteName", echoCtx.Request().URL.Path, http.StatusNotFound, err)
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "リソースが見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "UpdateIngressRouteName", echoCtx.Request().URL.Path, http.StatusInternalServerError, err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 204 No Content を返す
}

// DeleteIngressRoute は DELETE /ingress-routes/:id のハンドラー
func (ingressRouteHandler *IngressRouteHandler) DeleteIngressRoute(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string)      // ミドルウェアがセットした UserID を取得する
	ingressRouteID := echoCtx.Param("id")          // パスパラメータから ingress_route ID を取得する

	if err := ingressRouteHandler.ingressRouteService.DeleteIngressRoute(echoCtx.Request().Context(), userID, ingressRouteID); err != nil { // サービスを呼び出して ingress_route を削除する
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("IngressRouteHandler", "DeleteIngressRoute", echoCtx.Request().URL.Path, http.StatusForbidden, err)
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "アクセスが禁止されています"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("IngressRouteHandler", "DeleteIngressRoute", echoCtx.Request().URL.Path, http.StatusNotFound, err)
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "リソースが見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "DeleteIngressRoute", echoCtx.Request().URL.Path, http.StatusInternalServerError, err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 204 No Content を返す
}

// ApplyProject は POST /projects/:id/apply のハンドラー
func (ingressRouteHandler *IngressRouteHandler) ApplyProject(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	projectID := echoCtx.Param("id")         // パスパラメータから project ID を取得する

	applyResult, err := ingressRouteHandler.applyService.ApplyProject(echoCtx.Request().Context(), userID, projectID) // サービスを呼び出して Deployment・IngressRoute を一括 apply する
	if err != nil {                                                                                                    // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("IngressRouteHandler", "ApplyProject", echoCtx.Request().URL.Path, http.StatusForbidden, err)
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "アクセスが禁止されています"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // IngressRoute が存在しない場合は 404 を返す
			logger.PrintHandlerError("IngressRouteHandler", "ApplyProject", echoCtx.Request().URL.Path, http.StatusNotFound, err)
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "IngressRoute が見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "ApplyProject", echoCtx.Request().URL.Path, http.StatusInternalServerError, err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return echoCtx.JSON(http.StatusOK, applyResult) // 一括 apply の結果を返す
}

// GetProjectPendingSummary は GET /projects/:id/pending-summary のハンドラー
func (ingressRouteHandler *IngressRouteHandler) GetProjectPendingSummary(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string) // ミドルウェアがセットした UserID を取得する
	projectID := echoCtx.Param("id")         // パスパラメータから project ID を取得する

	summary, err := ingressRouteHandler.applyService.GetProjectPendingSummary(echoCtx.Request().Context(), userID, projectID) // サービスを呼び出して pending 件数を集計する
	if err != nil {                                                                                                             // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("IngressRouteHandler", "GetProjectPendingSummary", echoCtx.Request().URL.Path, http.StatusForbidden, err)
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "アクセスが禁止されています"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // project が存在しない場合は 404 を返す
			logger.PrintHandlerError("IngressRouteHandler", "GetProjectPendingSummary", echoCtx.Request().URL.Path, http.StatusNotFound, err)
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "リソースが見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "GetProjectPendingSummary", echoCtx.Request().URL.Path, http.StatusInternalServerError, err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.JSON(http.StatusOK, summary) // pending 集計結果を返す
}

// ListPathRules は GET /ingress-routes/:id/path-rules のハンドラー
func (ingressRouteHandler *IngressRouteHandler) ListPathRules(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string)   // ミドルウェアがセットした UserID を取得する
	ingressRouteID := echoCtx.Param("id")      // パスパラメータから ingress_route ID を取得する

	pathRuleList, err := ingressRouteHandler.ingressRouteService.ListPathRules(echoCtx.Request().Context(), userID, ingressRouteID) // サービスを呼び出して path_rule 一覧を取得する
	if err != nil {                                                                                                                   // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("IngressRouteHandler", "ListPathRules", echoCtx.Request().URL.Path, http.StatusForbidden, err)
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "アクセスが禁止されています"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // IngressRoute が存在しない場合は 404 を返す
			logger.PrintHandlerError("IngressRouteHandler", "ListPathRules", echoCtx.Request().URL.Path, http.StatusNotFound, err)
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "リソースが見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "ListPathRules", echoCtx.Request().URL.Path, http.StatusInternalServerError, err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.JSON(http.StatusOK, pathRuleList) // path_rule 一覧を返す
}

// CreatePathRule は POST /ingress-routes/:id/path-rules のハンドラー
func (ingressRouteHandler *IngressRouteHandler) CreatePathRule(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string)   // ミドルウェアがセットした UserID を取得する
	ingressRouteID := echoCtx.Param("id")      // パスパラメータから ingress_route ID を取得する

	var requestBody service.CreatePathRuleRequest                  // リクエストボディの構造体を定義する
	if err := echoCtx.Bind(&requestBody); err != nil {             // リクエストをバインドする
		logger.PrintHandlerError("IngressRouteHandler", "CreatePathRule", echoCtx.Request().URL.Path, http.StatusBadRequest, err)
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストが不正です"})
	}

	pathRuleData, err := ingressRouteHandler.ingressRouteService.CreatePathRule(echoCtx.Request().Context(), userID, ingressRouteID, requestBody) // サービスを呼び出して path_rule を作成する
	if err != nil {                                                                                                                                  // エラーが発生した場合
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("IngressRouteHandler", "CreatePathRule", echoCtx.Request().URL.Path, http.StatusForbidden, err)
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "アクセスが禁止されています"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // IngressRoute が存在しない場合は 404 を返す
			logger.PrintHandlerError("IngressRouteHandler", "CreatePathRule", echoCtx.Request().URL.Path, http.StatusNotFound, err)
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "リソースが見つかりません"})
		}
		if errors.Is(err, service.ErrDuplicatePathPrefix) { // 同じパスが既に存在する場合は 409 を返す
			logger.PrintHandlerError("IngressRouteHandler", "CreatePathRule", echoCtx.Request().URL.Path, http.StatusConflict, err)
			return echoCtx.JSON(http.StatusConflict, map[string]string{"error": "同じパスプレフィックスが既に登録されています"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "CreatePathRule", echoCtx.Request().URL.Path, http.StatusInternalServerError, err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.JSON(http.StatusCreated, pathRuleData) // 作成した path_rule を返す
}

// DeletePathRule は DELETE /ingress-routes/:id/path-rules/:pathRuleID のハンドラー
func (ingressRouteHandler *IngressRouteHandler) DeletePathRule(echoCtx echo.Context) error {
	userID := echoCtx.Get("UserID").(string)    // ミドルウェアがセットした UserID を取得する
	pathRuleID := echoCtx.Param("pathRuleID")   // パスパラメータから path_rule ID を取得する

	if err := ingressRouteHandler.ingressRouteService.DeletePathRule(echoCtx.Request().Context(), userID, pathRuleID); err != nil { // サービスを呼び出して path_rule を削除する
		if errors.Is(err, service.ErrForbidden) { // 所有者でない場合は 403 を返す
			logger.PrintHandlerError("IngressRouteHandler", "DeletePathRule", echoCtx.Request().URL.Path, http.StatusForbidden, err)
			return echoCtx.JSON(http.StatusForbidden, map[string]string{"error": "アクセスが禁止されています"})
		}
		if errors.Is(err, gorm.ErrRecordNotFound) { // レコードが存在しない場合は 404 を返す
			logger.PrintHandlerError("IngressRouteHandler", "DeletePathRule", echoCtx.Request().URL.Path, http.StatusNotFound, err)
			return echoCtx.JSON(http.StatusNotFound, map[string]string{"error": "リソースが見つかりません"})
		}
		logger.PrintHandlerError("IngressRouteHandler", "DeletePathRule", echoCtx.Request().URL.Path, http.StatusInternalServerError, err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"error": "内部サーバーエラー"})
	}
	return echoCtx.NoContent(http.StatusNoContent) // 204 No Content を返す
}
