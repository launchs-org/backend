package railpack

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestNewValidatesGitSource(t *testing.T) {
	// テストケースを定義する
	testCases := []struct {
		name        string
		config      BuildConfig
		expectError bool
	}{
		{
			name: "GitRepoが指定されていれば成功する",
			config: BuildConfig{
				GitRepo:          "https://github.com/org/repo",
				RegistryUsername: "robot-user",
				Namespace:        "buildkit",
				ImageName:        "my-app",
				ImageTag:         "v1.0.0",
			},
			expectError: false,
		},
		{
			name: "GitRepoが空の場合はエラーになる",
			config: BuildConfig{
				RegistryUsername: "robot-user",
				Namespace:        "buildkit",
				ImageName:        "my-app",
				ImageTag:         "v1.0.0",
			},
			expectError: true,
		},
	}

	fakeClientset := fake.NewSimpleClientset() // fake clientsetを生成する

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := New(fakeClientset, testCase.config) // クライアントを生成する
			if testCase.expectError && err == nil {
				t.Fatalf("エラーが返されるべきですが、nilが返されました")
			}
			if !testCase.expectError && err != nil {
				t.Fatalf("予期しないエラーが発生しました: %v", err)
			}
		})
	}
}

func TestNewValidatesArchiveSource(t *testing.T) {
	// テストケースを定義する
	testCases := []struct {
		name        string
		config      BuildConfig
		expectError bool
	}{
		{
			name: "ArchiveURL/ArchiveEncKeyHex/ArchiveSHA256Hexが揃っていれば成功する",
			config: BuildConfig{
				SourceType:       "archive",
				ArchiveURL:       "https://file.io/xxxx",
				ArchiveEncKeyHex: "abcd1234",
				ArchiveSHA256Hex: "ef567890",
				RegistryUsername: "robot-user",
				Namespace:        "buildkit",
				ImageName:        "my-app",
				ImageTag:         "v1.0.0",
			},
			expectError: false,
		},
		{
			name: "SourceType省略でもArchiveURLがあればarchiveとして自動判定され成功する",
			config: BuildConfig{
				ArchiveURL:       "https://file.io/xxxx",
				ArchiveEncKeyHex: "abcd1234",
				ArchiveSHA256Hex: "ef567890",
				RegistryUsername: "robot-user",
				Namespace:        "buildkit",
				ImageName:        "my-app",
				ImageTag:         "v1.0.0",
			},
			expectError: false,
		},
		{
			name: "ArchiveEncKeyHexが空の場合はエラーになる",
			config: BuildConfig{
				SourceType:       "archive",
				ArchiveURL:       "https://file.io/xxxx",
				ArchiveSHA256Hex: "ef567890",
				RegistryUsername: "robot-user",
				Namespace:        "buildkit",
				ImageName:        "my-app",
				ImageTag:         "v1.0.0",
			},
			expectError: true,
		},
		{
			name: "ArchiveSHA256Hexが空の場合はエラーになる",
			config: BuildConfig{
				SourceType:       "archive",
				ArchiveURL:       "https://file.io/xxxx",
				ArchiveEncKeyHex: "abcd1234",
				RegistryUsername: "robot-user",
				Namespace:        "buildkit",
				ImageName:        "my-app",
				ImageTag:         "v1.0.0",
			},
			expectError: true,
		},
	}

	fakeClientset := fake.NewSimpleClientset()

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := New(fakeClientset, testCase.config)
			if testCase.expectError && err == nil {
				t.Fatalf("エラーが返されるべきですが、nilが返されました")
			}
			if !testCase.expectError && err != nil {
				t.Fatalf("予期しないエラーが発生しました: %v", err)
			}
		})
	}
}
