package handler

import (
	"archive-server/storage"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

// ArchiveHandler はアーカイブの保存・配信・削除を行うHTTPハンドラー
type ArchiveHandler struct {
	fileStorage *storage.FileStorage // アーカイブの実体を保持するストレージ
}

// NewArchiveHandler は ArchiveHandler を生成する
func NewArchiveHandler(fileStorage *storage.FileStorage) *ArchiveHandler {
	return &ArchiveHandler{fileStorage: fileStorage} // ストレージを注入する
}

// Upload はリクエストボディをそのままアーカイブとして保存し、生成したIDを返す。
// 呼び出し元（handler サービス）はクラスタ内からのみアクセスするため、追加の認証は行わない
// （NetworkPolicy でクラスタ外部・buildkit 以外からの到達を遮断する前提）。
func (archiveHandler *ArchiveHandler) Upload(echoCtx echo.Context) error {
	id, err := archiveHandler.fileStorage.Save(echoCtx.Request().Body) // リクエストボディをそのまま保存する
	if err != nil {
		log.Printf("アーカイブの保存に失敗しました: %v", err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "アーカイブの保存に失敗しました",
		})
	}
	return echoCtx.JSON(http.StatusCreated, map[string]string{
		"id": id, // 保存したアーカイブのIDを返す
	})
}

// Download は指定IDのアーカイブをそのままストリーム配信し、配信完了後にアーカイブを削除する（ワンタイムダウンロード）
func (archiveHandler *ArchiveHandler) Download(echoCtx echo.Context) error {
	id := echoCtx.Param("id") // パスパラメータからIDを取得する

	file, err := archiveHandler.fileStorage.Open(id) // アーカイブファイルを開く
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) { // 期限切れ・存在しない場合は404を返す
			return echoCtx.JSON(http.StatusNotFound, map[string]string{
				"error": "アーカイブが見つかりません",
			})
		}
		log.Printf("アーカイブのオープンに失敗しました: %v", err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "内部サーバーエラー",
		})
	}
	echoCtx.Response().Header().Set(echo.HeaderContentType, "application/octet-stream") // バイナリとして配信する
	_, copyErr := io.Copy(echoCtx.Response(), file)
	file.Close() // 削除前にクローズする（Windowsでは開いたままだと削除できないため）

	if copyErr != nil {
		log.Printf("アーカイブの配信に失敗しました: %v", copyErr)
		return copyErr // 配信が途中で失敗した場合は削除せず残す（再ダウンロードの余地を残す）
	}

	if deleteErr := archiveHandler.fileStorage.Delete(id); deleteErr != nil { // ワンタイムダウンロードのため配信完了後に削除する
		log.Printf("配信完了後のアーカイブ削除に失敗しました: %v", deleteErr)
	}
	return nil
}

// Delete は指定IDのアーカイブを即時削除する（builderがダウンロード完了後に呼び出す想定）
func (archiveHandler *ArchiveHandler) Delete(echoCtx echo.Context) error {
	id := echoCtx.Param("id") // パスパラメータからIDを取得する

	if err := archiveHandler.fileStorage.Delete(id); err != nil {
		log.Printf("アーカイブの削除に失敗しました: %v", err)
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "アーカイブの削除に失敗しました",
		})
	}
	return echoCtx.NoContent(http.StatusNoContent)
}
