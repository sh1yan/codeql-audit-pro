//go:build windows

package assistant

import (
	"os/exec"
	"strconv"
	"syscall"
)

const createNewProcessGroup = 0x00000200

// setProcessGroup 在 Windows 上创建独立进程组，便于整棵进程树一起结束。
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}

// terminateProcessGroup 用 taskkill /T 请求结束进程树（不带 /F）。
func terminateProcessGroup(pid int) {
	_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T").Run()
}

// killProcessGroup 用 taskkill /T /F 强制结束进程树。
func killProcessGroup(pid int) {
	_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}
