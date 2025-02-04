package myapp

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func GetExecutablePath() (file string, err error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "darwin" {
		// 获取 .app 包路径
		appBundlePath := findAppBundlePath(exePath)
		if appBundlePath == "" {
			return "", fmt.Errorf("Error: Not running inside an .app bundle")
		}

		return appBundlePath, nil
	}

	return exePath, nil
}

func findAppBundlePath(exePath string) string {
	// 检查路径中是否包含 .app 目录
	dir := exePath
	for {
		if filepath.Ext(dir) == ".app" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir { // 到达根目录
			break
		}
		dir = parent
	}
	return ""
}
