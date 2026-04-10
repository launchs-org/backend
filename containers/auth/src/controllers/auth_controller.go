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

// Login ログイン処理を行うエンドポイント
func (controller *AuthController) Login(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "login logic here"})
}

// ValidateToken トークンの有効性を確認するエンドポイント
func (controller *AuthController) ValidateToken(echoContext echo.Context) error {
	return echoContext.JSON(http.StatusOK, echo.Map{"message": "validate token logic here"})
}
