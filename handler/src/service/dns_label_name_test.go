package service

import "testing"

// TestValidateDNSLabelName_日本語は拒否される は日本語を含む名前がエラーになることを確認する
func TestValidateDNSLabelName_日本語は拒否される(t *testing.T) {
	invalidNameList := []string{"プロジェクト", "test-アプリ", "テスト123"} // 日本語を含む名前一覧を定義する
	for _, invalidName := range invalidNameList {
		if err := validateDNSLabelName(invalidName); err != ErrInvalidResourceName { // エラーが返ることを確認する
			t.Errorf("名前 %q は拒否されるべきですが、エラー: %v", invalidName, err)
		}
	}
}

// TestValidateDNSLabelName_DNSラベル形式は許可される は英小文字・数字・ハイフンのみの名前が許可されることを確認する
func TestValidateDNSLabelName_DNSラベル形式は許可される(t *testing.T) {
	validNameList := []string{"my-app", "app123", "a", "test-deploy-1"} // 有効な名前一覧を定義する
	for _, validName := range validNameList {
		if err := validateDNSLabelName(validName); err != nil { // エラーが返らないことを確認する
			t.Errorf("名前 %q は許可されるべきですが、エラー: %v", validName, err)
		}
	}
}

// TestValidateDNSLabelName_大文字や記号や先頭末尾ハイフンは拒否される は DNS ラベル形式に違反する名前がエラーになることを確認する
func TestValidateDNSLabelName_大文字や記号や先頭末尾ハイフンは拒否される(t *testing.T) {
	invalidNameList := []string{"My-App", "app_name", "-app", "app-", "app name", ""} // 無効な名前一覧を定義する
	for _, invalidName := range invalidNameList {
		if err := validateDNSLabelName(invalidName); err != ErrInvalidResourceName { // エラーが返ることを確認する
			t.Errorf("名前 %q は拒否されるべきですが、エラー: %v", invalidName, err)
		}
	}
}
