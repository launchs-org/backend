package storage

import (
	"strings"
	"testing"
)

// TestFileStorage_SaveAndOpen は保存したアーカイブを正しく読み込めることを確認する
func TestFileStorage_SaveAndOpen(t *testing.T) {
	tempDir := t.TempDir()                    // テスト用の一時ディレクトリを作成する
	fileStorage := NewFileStorage(tempDir)     // ファイルストレージを生成する
	content := "encrypted-archive-content"    // テスト用のダミーコンテンツ

	id, err := fileStorage.Save(strings.NewReader(content)) // コンテンツを保存する
	if err != nil {
		t.Fatalf("Save() がエラーを返しました: %v", err) // 保存失敗時はテスト失敗とする
	}

	file, err := fileStorage.Open(id) // 保存したファイルを開く
	if err != nil {
		t.Fatalf("Open() がエラーを返しました: %v", err) // オープン失敗時はテスト失敗とする
	}
	defer file.Close()

	buf := make([]byte, len(content))
	if _, err := file.Read(buf); err != nil {
		t.Fatalf("ファイルの読み込みに失敗しました: %v", err) // 読み込み失敗時はテスト失敗とする
	}
	if string(buf) != content {
		t.Fatalf("読み込んだ内容が一致しません: got=%s, want=%s", string(buf), content) // 内容不一致時はテスト失敗とする
	}
}

// TestFileStorage_OpenNotFound は存在しないIDを開こうとした場合にErrNotFoundを返すことを確認する
func TestFileStorage_OpenNotFound(t *testing.T) {
	tempDir := t.TempDir()
	fileStorage := NewFileStorage(tempDir)

	_, err := fileStorage.Open("nonexistent-id") // 存在しないIDでオープンを試みる
	if err != ErrNotFound {
		t.Fatalf("ErrNotFound が返るべきですが %v が返りました", err) // 期待するエラーでない場合はテスト失敗とする
	}
}

// TestFileStorage_Delete は保存したアーカイブを削除できることを確認する
func TestFileStorage_Delete(t *testing.T) {
	tempDir := t.TempDir()
	fileStorage := NewFileStorage(tempDir)

	id, err := fileStorage.Save(strings.NewReader("data")) // コンテンツを保存する
	if err != nil {
		t.Fatalf("Save() がエラーを返しました: %v", err)
	}

	if err := fileStorage.Delete(id); err != nil {
		t.Fatalf("Delete() がエラーを返しました: %v", err) // 削除失敗時はテスト失敗とする
	}

	_, err = fileStorage.Open(id) // 削除後にオープンを試みる
	if err != ErrNotFound {
		t.Fatalf("削除後は ErrNotFound が返るべきですが %v が返りました", err) // 削除されていない場合はテスト失敗とする
	}
}

// TestFileStorage_DeleteNonexistent は存在しないIDを削除してもエラーにならないことを確認する（冪等性）
func TestFileStorage_DeleteNonexistent(t *testing.T) {
	tempDir := t.TempDir()
	fileStorage := NewFileStorage(tempDir)

	if err := fileStorage.Delete("nonexistent-id"); err != nil {
		t.Fatalf("存在しないIDの削除はエラーになるべきではありません: %v", err) // 冪等でない場合はテスト失敗とする
	}
}
