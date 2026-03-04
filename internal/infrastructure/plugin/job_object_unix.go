//go:build !windows

package plugin

func initProcessJobObject() uintptr {
	return 0
}

func addProcessToJobObject(_ uintptr, _ int) {}
