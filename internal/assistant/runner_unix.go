//go:build !windows

package assistant

import (
	"os/exec"
	"syscall"
)

// setProcessGroup 让 CodeQL 及其子进程（Maven、Gradle、编译器）进入同一个进程组。
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcessGroup 向整个进程组发送 SIGINT，争取优雅退出。
func terminateProcessGroup(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGINT)
}

// killProcessGroup 强制结束整个进程组。
func killProcessGroup(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
