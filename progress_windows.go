//go:build windows
// +build windows

package myapp

import (
	"golang.org/x/sys/windows"
	"os/exec"
)

func StartProcessDetached(execFile string) error {
	cmd := exec.Command(execFile)
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}

	err := cmd.Start()
	if err != nil {
		return err
	}

	return nil
}
