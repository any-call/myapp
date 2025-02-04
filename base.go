package myapp

import (
	"fmt"
	"github.com/any-call/gobase/util/mycrypto"
	"github.com/any-call/gobase/util/mynet"
	"github.com/any-call/gobase/util/myos"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
)

type UpgradeStep int

const (
	StepDownload       UpgradeStep = 1  //下载
	StepDownloadOK     UpgradeStep = 10 //下载
	StepDownloadFail   UpgradeStep = 11 //下载
	StepRename         UpgradeStep = 2  //重命令当前进程文件
	StepUnzip          UpgradeStep = 3  //解压下载文件
	StepRemoveDownload UpgradeStep = 4  //删除下载文件
	StepRestart        UpgradeStep = 5  //重启下截文件
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

func ClearBackupAPP() {
	//首先删掉旧的升级back
	execFile, err := GetExecutablePath()
	if err != nil {
		return
	}

	removeFile := execFile + "_backup"
	if myos.IsMac() {
		_ = os.RemoveAll(removeFile)
	} else if myos.IsWin() {
		_ = os.Remove(removeFile)
	}

	return
}

// 升级APP : 下载 -> 重命令当前进程文件 -> 解压 到当前进程位置 -> 删除下载文件 -> 重启下截文件 -> 退出当前进程
func UpgradeApp(downloadUrl string, dlProcessCb func(percent float64, step UpgradeStep)) error {
	tempDir := os.TempDir()
	saveFile := filepath.Join(tempDir, path.Base(downloadUrl))
	if err := mynet.DownloadFile(downloadUrl, saveFile, func(r *http.Request) {
		r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	}, func(readLen, totalLen int64) {
		if dlProcessCb != nil {
			if totalLen > 0 {
				dlProcessCb(float64(readLen)/float64(totalLen), StepDownload)
			} else {
				dlProcessCb(float64(readLen)/float64(readLen+100), StepDownload)
			}
		}
	}); err != nil {
		if dlProcessCb != nil {
			dlProcessCb(1.0, StepDownloadFail)
		}
		return err
	} else {
		if dlProcessCb != nil {
			dlProcessCb(1.0, StepDownloadOK)
		}
	}

	if dlProcessCb != nil {
		dlProcessCb(1.0, StepRename)
	}
	//将主程序自动命令为 _old.exe
	execFile, newFile, err := renameSelf()
	if err != nil {
		return err
	}

	if dlProcessCb != nil {
		dlProcessCb(1.0, StepUnzip)
	}
	//解压文件到当前目录
	if err := mycrypto.Unzip(saveFile, filepath.Dir(newFile)); err != nil {
		return err
	}

	if dlProcessCb != nil {
		dlProcessCb(1.0, StepRemoveDownload)
	}
	//删除下载的文件
	_ = os.Remove(saveFile)

	if dlProcessCb != nil {
		dlProcessCb(1.0, StepRestart)
	}
	////重新启动新的进程
	if err := StartProcessDetached(execFile); err != nil {
		return err
	}

	return nil
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

func renameSelf() (oldFile, newFile string, err error) {
	execPath, err := GetExecutablePath()
	if err != nil {
		return "", "", err
	}

	if myos.IsMac() {
		// 获取 .app 包路径
		newAppBundlePath := execPath + "_backup"

		// 重命名 .app 包
		err := os.Rename(execPath, newAppBundlePath)
		if err != nil {
			return "", "", fmt.Errorf("Error renaming .app bundle:%v", err.Error())
		}

		return execPath, newAppBundlePath, nil
	} else if myos.IsWin() {
		// 构建新的文件名
		newPath := execPath + "_backup"
		// 重命名当前应用程序
		err = os.Rename(execPath, newPath)
		if err != nil {
			return "", "", err
		}

		return execPath, newPath, nil
	}

	return "", "", fmt.Errorf("rename fail")
}
