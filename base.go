package myapp

import (
	"fmt"
	"github.com/any-call/gobase/util/mycrypto"
	"github.com/any-call/gobase/util/mynet"
	"github.com/any-call/gobase/util/myos"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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

func ClearBackupAPP(t time.Duration) {
	//首先删掉旧的升级back
	execFile, err := GetExecutablePath()
	if err != nil {
		return
	}

	removeFile := execFile + "_backup"
	if myos.IsMac() {
		if myos.IsExistDir(removeFile) {
			time.Sleep(t)
			_ = os.RemoveAll(removeFile)
		}
	} else if myos.IsWin() || myos.IsAndroid() || myos.IsLinux() {
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
	currExecPackage, _, err := renameSelf()
	if err != nil {
		return err
	}

	if dlProcessCb != nil {
		dlProcessCb(1.0, StepUnzip)
	}

	unzipFolder := filepath.Join(filepath.Dir(saveFile), fmt.Sprintf("%d", time.Now().UnixMilli())) //filepath.Dir(newFile)

	defer func() {
		//删除下载的文件 与 旧的文件
		_ = os.Remove(saveFile)
		_ = os.Remove(unzipFolder)
	}()

	//解压文件到当前目录
	if err := mycrypto.Unzip(saveFile, unzipFolder); err != nil {
		return err
	}

	//解压目录下的文件
	var unzipBinFile string
	filepath.Walk(unzipFolder, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if myos.IsMac() {
			// 只查找 .app 且是目录
			if info.IsDir() && strings.HasSuffix(info.Name(), ".app") {
				unzipBinFile = path
				// 如果只想要顶层目录，不要递归深入 .app 内部
				return nil
			}
		} else if myos.IsWin() {
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".exe") {
				unzipBinFile = path
				return nil
			}
		} else if myos.IsAndroid() || myos.IsLinux() {
			if !info.IsDir() {
				unzipBinFile = path
				return nil
			}
		}

		return nil
	})

	if unzipBinFile == "" {
		return fmt.Errorf("not find bininary file")
	}

	if myos.IsMac() || myos.IsWin() || myos.IsAndroid() { //将一个Mac 移到当前执行目录，
		if err := os.Rename(unzipBinFile, currExecPackage); err != nil {
			return err
		}
	} else if myos.IsLinux() {
		if err := copyFile(unzipBinFile, currExecPackage); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("unsupport platform")
	}

	if dlProcessCb != nil {
		dlProcessCb(1.0, StepRemoveDownload)
	}

	if dlProcessCb != nil {
		dlProcessCb(1.0, StepRestart)
	}

	execBinFile, err := os.Executable()
	if err != nil {
		return err
	}

	if myos.IsAndroid() || myos.IsLinux() { //android 中 取到的执行路径是 改名后的
		execBinFile = currExecPackage
	}

	////重新启动新的进程
	if err := StartProcessDetached(execBinFile); err != nil {
		return err
	} else {
		os.Exit(1) //当成功启动后退出当前进程
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

		//检测文件是否存在，
		if myos.IsExistDir(newAppBundlePath) {
			if err := os.RemoveAll(newAppBundlePath); err != nil {
				return "", "", err
			}
		}

		// 重命名 .app 包
		err := os.Rename(execPath, newAppBundlePath)
		if err != nil {
			return "", "", fmt.Errorf("Error renaming .app bundle:%v", err.Error())
		}

		return execPath, newAppBundlePath, nil
	} else if myos.IsWin() || myos.IsAndroid() || myos.IsLinux() {
		// 构建新的文件名
		newPath := execPath + "_backup"

		//检测文件是否存在，
		if myos.IsExistFile(newPath) {
			if err := os.Remove(newPath); err != nil {
				return "", "", err
			}
		}

		// 重命名当前应用程序
		err = os.Rename(execPath, newPath)
		if err != nil {
			return "", "", err
		}

		return execPath, newPath, nil
	}

	return "", "", fmt.Errorf("rename fail")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	// 获取原文件的权限
	fi, err := in.Stat()
	if err != nil {
		return err
	}

	// 设置目标文件权限，保留原权限（包括可执行）
	if err = os.Chmod(dst, fi.Mode()); err != nil {
		return err
	}
	// 删除源文件
	return os.Remove(src)
}
