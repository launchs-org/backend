package logger

import (
	"fmt"
	"log"
	"runtime"
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
	printline()

	// ファイル名と行番号を出力
	log.Println(fmt.Sprintf("Code Error: %s:%d", fileName, line))
	for _, val := range vals {
		log.Println(val)
	}

	// ラインを出す
	printline()
}

func printline() {
	log.Println("")
	log.Println("--------------------------------------------------")
}
