package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// ErrNotFound はアーカイブが存在しない場合のエラー
var ErrNotFound = errors.New("アーカイブが見つかりません")

// FileStorage は共有ボリューム上にアーカイブファイルを保存・配信するストレージ
type FileStorage struct {
	baseDir string // アーカイブ保存先ディレクトリ
}

// NewFileStorage は FileStorage を生成する。保存先ディレクトリが存在しなければ作成する
func NewFileStorage(baseDir string) *FileStorage {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		panic("アーカイブ保存先ディレクトリの作成に失敗しました: " + err.Error()) // 起動時に必須の前提条件のため即座に落とす
	}
	return &FileStorage{baseDir: baseDir}
}

// Save はアーカイブデータを新規ファイルとして保存し、ランダムなIDを返す
func (fileStorage *FileStorage) Save(data io.Reader) (id string, err error) {
	id = uuid.NewString() // 推測困難なランダムIDを生成する

	destFile, createErr := os.Create(fileStorage.path(id)) // 保存先ファイルを作成する
	if createErr != nil {
		return "", createErr
	}
	defer destFile.Close()

	if _, copyErr := io.Copy(destFile, data); copyErr != nil { // アーカイブ本体を書き込む
		os.Remove(fileStorage.path(id)) // 書き込み失敗時は不完全なファイルを削除する
		return "", copyErr
	}

	return id, nil
}

// Open は指定IDのアーカイブを読み込み用に開く
func (fileStorage *FileStorage) Open(id string) (*os.File, error) {
	file, err := os.Open(fileStorage.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return file, nil
}

// Delete は指定IDのアーカイブを削除する。存在しない場合もエラーにしない（冪等）
func (fileStorage *FileStorage) Delete(id string) error {
	err := os.Remove(fileStorage.path(id))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// BaseDir は保存先ディレクトリを返す（クリーナーからの走査用）
func (fileStorage *FileStorage) BaseDir() string {
	return fileStorage.baseDir
}

// path は指定IDに対応するファイルパスを組み立てる
func (fileStorage *FileStorage) path(id string) string {
	return filepath.Join(fileStorage.baseDir, id)
}
