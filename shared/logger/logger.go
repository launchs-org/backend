package logger

import (
	"fmt"
	"log"
	"runtime"
)

func Println(vals ...interface{}) {
	_, fileName, line, ok := runtime.Caller(1)
	if !ok {
		fmt.Println("logger: failed to get caller")
		return
	}
	printline()
	log.Print(fmt.Sprintf("Print Info: %s:%d", fileName, line))
	for _, val := range vals {
		log.Println(val)
	}
	printline()
}

func PrintErr(vals ...interface{}) {
	_, fileName, line, ok := runtime.Caller(1)
	if !ok {
		fmt.Println("logger: failed to get caller")
		return
	}
	printline()
	log.Println(fmt.Sprintf("Code Error: %s:%d", fileName, line))
	for _, val := range vals {
		log.Println(val)
	}
	printline()
}

func printline() {
	log.Println("")
	log.Println("--------------------------------------------------")
}
