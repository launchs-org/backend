package controllers

import (
	"net/http"

	"auth/services"

	"github.com/labstack/echo/v4"
)

// AuthController 認証関連のリクエストを処理するコントローラー
type AuthController struct {
	authService services.AuthService
}

// NewAuthController AuthController の新しいインスタンスを作成する
func NewAuthController(service services.AuthService) *AuthController {
	return &AuthController{
		authService: service,
	}
}

// Login ログインリクエストを受け取り、AuthService を呼び出して認証結果を返します。
func (controller *AuthController) Login(echoContext echo.Context) error {
	// リクエストボディからユーザー名とパスワードを取得
	type loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	req := new(loginRequest)
	if err := echoContext.Bind(req); err != nil {
		return echoContext.JSON(http.StatusBadRequest, echo.Map{"error": "リクエスト形式が不正です"})
	}

	// 認証処理を実行
	token, err := controller.authService.Login(req.Username, req.Password)
	if err != nil {
		// 認証失敗時は401 Unauthorizedを返す
		return echoContext.JSON(http.StatusUnauthorized, echo.Map{"error": err.Error()})
	}

	// 認証成功時、トークンを返す
	return echoContext.JSON(http.StatusOK, echo.Map{"token": token})
}

// ValidateToken JWT トークンの検証を行い、結果を返します。
func (controller *AuthController) ValidateToken(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "validate token logic here"})
}
