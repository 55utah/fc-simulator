package nes

import "fmt"

func Logger(format string, a ...interface{}) {
	fmt.Printf(format, a...)
	fmt.Printf("\n")
}
