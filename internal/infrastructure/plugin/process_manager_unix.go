//go:build !windows

package plugin

import "os/exec"

func setSysProcAttr(cmd *exec.Cmd) {
	// 非Windows環境では何もしない
}
