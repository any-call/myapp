//go:build wasm
// +build wasm

package myapp

import (
	"fmt"
)

func StartProcessDetached(execFile string) error {
	return fmt.Errorf("Web平台不支持进程控制")
}
