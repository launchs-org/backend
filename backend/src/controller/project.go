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
