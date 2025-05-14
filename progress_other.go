//go:build linux || darwin || android
// +build linux darwin android

package myapp

import (
	"os/exec"
	"syscall"
)

func StartProcessDetached(execFile string) error {
	cmd := exec.Command(execFile)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	err := cmd.Start()
	if err != nil {
		return err
	}
	return nil
}
