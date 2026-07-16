package middlewares

import (
	"testing"
	"time"
)

func TestIssueArchiveUploadTokenValidateRoundTrip(t *testing.T) {
	SetArchiveUploadTokenSecretForTest([]byte("test-secret-key-for-archive-upload-token")) // テスト用シークレットを設定する

	claim := ArchiveUploadTokenClaim{
		DeploymentID: "deployment-123",
		ArchiveURL:   "https://file.io/testlink",
		EncKeyHex:    "abcd1234",
		SHA256Hex:    "ef567890",
		FileName:     "source.tar.gz",
		SizeBytes:    12345,
	}

	tokenString, err := IssueArchiveUploadToken(claim) // トークンを発行する
	if err != nil {
		t.Fatalf("IssueArchiveUploadTokenが失敗しました: %v", err)
	}

	validatedClaim, err := ValidateArchiveUploadToken(tokenString) // トークンを検証する
	if err != nil {
		t.Fatalf("ValidateArchiveUploadTokenが失敗しました: %v", err)
	}

	if validatedClaim.DeploymentID != claim.DeploymentID {
		t.Errorf("DeploymentIDが一致しません: got=%s want=%s", validatedClaim.DeploymentID, claim.DeploymentID)
	}
	if validatedClaim.ArchiveURL != claim.ArchiveURL {
		t.Errorf("ArchiveURLが一致しません: got=%s want=%s", validatedClaim.ArchiveURL, claim.ArchiveURL)
	}
	if validatedClaim.EncKeyHex != claim.EncKeyHex {
		t.Errorf("EncKeyHexが一致しません: got=%s want=%s", validatedClaim.EncKeyHex, claim.EncKeyHex)
	}
	if validatedClaim.SHA256Hex != claim.SHA256Hex {
		t.Errorf("SHA256Hexが一致しません: got=%s want=%s", validatedClaim.SHA256Hex, claim.SHA256Hex)
	}
	if validatedClaim.FileName != claim.FileName {
		t.Errorf("FileNameが一致しません: got=%s want=%s", validatedClaim.FileName, claim.FileName)
	}
	if validatedClaim.SizeBytes != claim.SizeBytes {
		t.Errorf("SizeBytesが一致しません: got=%d want=%d", validatedClaim.SizeBytes, claim.SizeBytes)
	}
}

func TestIssueArchiveUploadTokenSetsShortExpiry(t *testing.T) {
	SetArchiveUploadTokenSecretForTest([]byte("test-secret-key-for-archive-upload-token"))

	claim := ArchiveUploadTokenClaim{DeploymentID: "deployment-123"}
	tokenString, err := IssueArchiveUploadToken(claim)
	if err != nil {
		t.Fatalf("IssueArchiveUploadTokenが失敗しました: %v", err)
	}

	validatedClaim, err := ValidateArchiveUploadToken(tokenString)
	if err != nil {
		t.Fatalf("ValidateArchiveUploadTokenが失敗しました: %v", err)
	}

	expiresAt := validatedClaim.ExpiresAt.Time
	issuedAt := validatedClaim.IssuedAt.Time
	if expiresAt.Sub(issuedAt) != archiveUploadTokenExpiry { // 有効期限が15分固定であることを確認する
		t.Fatalf("有効期限が想定と異なります: got=%s want=%s", expiresAt.Sub(issuedAt), archiveUploadTokenExpiry)
	}
	if !expiresAt.After(time.Now()) { // 発行直後は未失効であることを確認する
		t.Fatalf("発行直後のトークンが既に失効しています")
	}
}

func TestValidateArchiveUploadTokenInvalidSignature(t *testing.T) {
	SetArchiveUploadTokenSecretForTest([]byte("secret-a"))
	claim := ArchiveUploadTokenClaim{DeploymentID: "deployment-123"}
	tokenString, err := IssueArchiveUploadToken(claim)
	if err != nil {
		t.Fatalf("IssueArchiveUploadTokenが失敗しました: %v", err)
	}

	SetArchiveUploadTokenSecretForTest([]byte("secret-b")) // 異なるシークレットに切り替える

	if _, err := ValidateArchiveUploadToken(tokenString); err == nil {
		t.Fatalf("異なるシークレットで署名検証が通過してしまいました")
	}
}
