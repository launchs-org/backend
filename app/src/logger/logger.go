package logger

import (
	"fmt"
	"log"
	"runtime"
)

// ANSIカラーコード
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

func Println(vals ...interface{}) {
	// 呼び出された場所を取得
	_, fileName, line, ok := runtime.Caller(1)
	if !ok {
		fmt.Println("logger: failed to get caller")
		return
	}

	// ラインを出す
	printline()

	// ファイル名と行番号を出力
	log.Print(fmt.Sprintf("Print Info: %s:%d", fileName, line))
	for _, val := range vals {
		log.Println(val)
	}

	// ラインを出す
	printline()
}

func PrintErr(vals ...interface{}) {
	// 呼び出された場所を取得
	_, fileName, line, ok := runtime.Caller(1)
	if !ok {
		fmt.Println("logger: failed to get caller")
		return
	}

	// ラインを出す
	printErrLine()

	// ファイル名と行番号をエラー色で出力
	log.Println(fmt.Sprintf("%s[ERROR] Code Error: %s:%d%s", colorRed, fileName, line, colorReset))
	for _, val := range vals {
		log.Println(fmt.Sprintf("%s%v%s", colorRed, val, colorReset))
	}

	// ラインを出す
	printErrLine()
}

// PrintHandlerError はハンドラー層でのエラーをフォーマット付きで出力する
// handler: ハンドラー名（例: "DeploymentHandler"）
// action: 操作名（例: "CreateDeployment"）
// path: リクエストパス（例: "/api/v1/deployments/abc"）
// statusCode: HTTPステータスコード（例: 500）
// err: 発生したエラー
func PrintHandlerError(handler, action, path string, statusCode int, err error) {
	// 呼び出された場所を取得
	_, fileName, line, ok := runtime.Caller(1)
	if !ok {
		fmt.Println("logger: failed to get caller")
		return
	}

	// ステータスコードに応じてカラーを切り替える
	statusColor := colorRed    // デフォルトは赤（500系）
	if statusCode >= 400 && statusCode < 500 {
		statusColor = colorYellow // 400系はオレンジ系（黄）
	}

	// ラインを出す
	printErrLine()

	// ハンドラー情報とエラーを出力する
	log.Println(fmt.Sprintf("%s[ERROR] %s#%s | %s:%d%s", colorRed, handler, action, fileName, line, colorReset))
	log.Println(fmt.Sprintf("%sPath       : %s%s", colorCyan, path, colorReset))
	log.Println(fmt.Sprintf("%sStatusCode : %d%s", statusColor, statusCode, colorReset))
	log.Println(fmt.Sprintf("%sError      : %v%s", colorRed, err, colorReset))

	// ラインを出す
	printErrLine()
}

func printline() {
	log.Println("")
	log.Println("--------------------------------------------------")
}

func printErrLine() {
	log.Println("")
	log.Println(fmt.Sprintf("%s==================================================%s", colorRed, colorReset))
}
