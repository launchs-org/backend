package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JobTokenClaim struct {
	JobID     string
	ImageName string
	ImageTag  string
}

func GenerateJobToken(claim JobTokenClaim) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"JobID":     claim.JobID,
		"ImageTag":  claim.ImageTag,
		"ImageName": claim.ImageName,
		"exp":       time.Now().Add(time.Minute * 10).Unix(),
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("SessionSecret")))
	return tokenString, err
}

func VerifyJobToken(tokenString string) (JobTokenClaim, error) {
	var claim JobTokenClaim
	secretKey := []byte(os.Getenv("SessionSecret"))

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return secretKey, nil
	})

	if err != nil {
		return claim, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if v, ok := claims["JobID"].(string); ok {
			claim.JobID = v
		}
		if v, ok := claims["ImageName"].(string); ok {
			claim.ImageName = v
		}
		if v, ok := claims["ImageTag"].(string); ok {
			claim.ImageTag = v
		}
		return claim, nil
	}

	return claim, jwt.ErrSignatureInvalid
}
