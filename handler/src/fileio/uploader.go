package fileio

import (
	"context"
	"io"
)

// Uploader はアーカイブを一時的な保管場所にアップロードし、ダウンロードURLを返すインターフェース。
// 開発環境では外部の litterbox サービスを使う FileIOClient、本番環境ではクラスタ内の
// archive-server を使う ArchiveServerClient がこれを実装する。
type Uploader interface {
	Upload(ctx context.Context, fileName string, data io.Reader, size int64) (downloadURL string, err error)
}
