module builder

go 1.26.2

require (
	launchs/shared v0.0.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/go-containerregistry v0.21.5
	github.com/labstack/echo/v5 v5.1.0
)

replace launchs/shared => /shared

tool github.com/air-verse/air
