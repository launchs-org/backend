package controller

import (
	"backend/service" // サービスパッケージをインポート
	"net/http"        // HTTPステータスコード用
	"github.com/labstack/echo/v5" // Echoフレームワーク
)

// CreateProjectRequest はプロジェクト作成のリクエストボディです
type CreateProjectRequest struct {
	Name string `json:"name"` // プロジェクト名
}

// CreateProject はプロジェクトを作成するエンドポイントのハンドラーです
func CreateProject(ctx *echo.Context) error {
	// リクエストボディを格納する変数を宣言
	var req CreateProjectRequest
	// リクエストボディを構造体にバインド
	if err := (*ctx).Bind(&req); err != nil {
		// バインドに失敗した場合は400エラーを返す
		return (*ctx).JSON(http.StatusBadRequest, map[string]string{
			"code":    "BAD_REQUEST", // エラーコード
			"message": "リクエストパラメータが不正です", // エラーメッセージ
		})
	}

	// コンテキストからユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	// 取得に失敗した場合
	if !ok {
		// 認証エラーとして401を返す
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED", // エラーコード
			"message": "認証に失敗しました", // エラーメッセージ
		})
	}

	// プロジェクト作成サービスを呼び出す
	project, err := service.CreateProject((*ctx).Request().Context(), service.CreateProjectInput{
		Name:    req.Name, // プロジェクト名
		OwnerID: userID,   // 所有者ID
	})

	// サービス実行中にエラーが発生した場合
	if err != nil {
		// エラーの種類に応じてステータスコードを分ける
		switch err {
		case service.ErrInvalidProjectName:
			// 不正なプロジェクト名の場合は400を返す
			return (*ctx).JSON(http.StatusBadRequest, map[string]string{
				"code":    "BAD_REQUEST", // エラーコード
				"message": "プロジェクト名が空か、不正な文字を含んでいます", // エラーメッセージ
			})
		case service.ErrProjectAlreadyExists:
			// 既に存在する場合は409を返す
			return (*ctx).JSON(http.StatusConflict, map[string]string{
				"code":    "CONFLICT", // エラーコード
				"message": "そのプロジェクト名は既に使われています", // エラーメッセージ
			})
		default:
			// その他のエラーは500を返す
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR", // エラーコード
				"message": err.Error(),       // エラー内容
			})
		}
	}

	// 正常に作成された場合は201ステータスとデータを返す
	return (*ctx).JSON(http.StatusCreated, map[string]interface{}{
		"data": project, // 作成されたプロジェクトデータ
	})
}
// GetProject はプロジェクトの詳細を取得するエンドポイントのハンドラーです
func GetProject(ctx *echo.Context) error {
	// パスパラメータからプロジェクトIDを取得
	id := (*ctx).Param("id")

	// コンテキストからユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	// 取得に失敗した場合
	if !ok {
		// 認証エラーとして401を返す
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED", // エラーコード
			"message": "認証に失敗しました", // エラーメッセージ
		})
	}

	// プロジェクト取得サービスを呼び出す
	project, err := service.GetProjectByID((*ctx).Request().Context(), id, userID)

	// サービス実行中にエラーが発生した場合
	if err != nil {
		// エラーの種類に応じてステータスコードを分ける
		switch err {
		case service.ErrProjectNotFound:
			// 見つからない場合は404を返す
			return (*ctx).JSON(http.StatusNotFound, map[string]string{
				"code":    "NOT_FOUND", // エラーコード
				"message": "プロジェクトが見つかりません", // エラーメッセージ
			})
		case service.ErrForbidden:
			// 権限がない場合は403を返す
			return (*ctx).JSON(http.StatusForbidden, map[string]string{
				"code":    "FORBIDDEN", // エラーコード
				"message": "このプロジェクトへのアクセス権限がありません", // エラーメッセージ
			})
		default:
			// その他のエラーは500を返す
			return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
				"code":    "INTERNAL_ERROR", // エラーコード
				"message": err.Error(),       // エラー内容
			})
		}
	}

	// 正常に取得できた場合は200ステータスとデータを返す
	return (*ctx).JSON(http.StatusOK, map[string]interface{}{
		"data": project, // プロジェクトデータ
	})
}

// ListProjects はプロジェクト一覧を取得するエンドポイントのハンドラーです
func ListProjects(ctx *echo.Context) error {
	// コンテキストからユーザーIDを取得
	userID, ok := (*ctx).Get("UserID").(string)
	// 取得に失敗した場合
	if !ok {
		// 認証エラーとして401を返す
		return (*ctx).JSON(http.StatusUnauthorized, map[string]string{
			"code":    "UNAUTHORIZED", // エラーコード
			"message": "認証に失敗しました", // エラーメッセージ
		})
	}

	// プロジェクト一覧取得サービスを呼び出す
	projects, err := service.ListProjects((*ctx).Request().Context(), userID)

	// サービス実行中にエラーが発生した場合
	if err != nil {
		// 500エラーを返す
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
			"code":    "INTERNAL_ERROR", // エラーコード
			"message": err.Error(),       // エラー内容
		})
	}

	// 正常に取得できた場合は200ステータスとデータを返す
	// デザインに合わせて items と total を含む形式にする
	return (*ctx).JSON(http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"items": projects,       // プロジェクト一覧
			"total": len(projects), // 合計件数
		},
	})
}

// プロジェクトを削除するコントローラー
func DeleteProject(ctx *echo.Context) error {
	// パスパラメータからプロジェクトIDを取得
	id := (*ctx).Param("id")

	// コンテキストからユーザーIDを取得
	OwnerID := (*ctx).Get("UserID").(string)

	// プロジェクト削除サービスを呼び出す
	err := service.DeleteProject((*ctx).Request().Context(), id, OwnerID)

	// サービス実行中にエラーが発生した場合
	if err != nil {
		// 500エラーを返す
		return (*ctx).JSON(http.StatusInternalServerError, map[string]string{
			"code":    "INTERNAL_ERROR", // エラーコード
			"message": err.Error(),       // エラー内容
		})
	}

	// 正常に削除できた場合は204ステータスを返す
	return (*ctx).NoContent(http.StatusNoContent)
}