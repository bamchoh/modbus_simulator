package plugin

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows Job Object 関連の定数・構造体定義
// ホストプロセス終了時に子プロセスを自動終了させるために使用する
const (
	jobObjectExtendedLimitInformationClass uint32 = 9
	jobObjectLimitKillOnJobClose           uint32 = 0x00002000
)

type jobobjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobobjectExtendedLimitInformation struct {
	BasicLimitInformation jobobjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

var (
	modkernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procSetInformationJobObject = modkernel32.NewProc("SetInformationJobObject")
)

// initProcessJobObject は JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE フラグ付きの
// Windows Job Object を作成して返す。失敗時は 0 を返す。
func initProcessJobObject() uintptr {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] JobObject 作成失敗: %v\n", err)
		return 0
	}

	info := jobobjectExtendedLimitInformation{
		BasicLimitInformation: jobobjectBasicLimitInformation{
			LimitFlags: jobObjectLimitKillOnJobClose,
		},
	}
	r, _, err := procSetInformationJobObject.Call(
		uintptr(job),
		uintptr(jobObjectExtendedLimitInformationClass),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
	)
	if r == 0 {
		fmt.Fprintf(os.Stderr, "[WARN] SetInformationJobObject 失敗: %v\n", err)
		_ = windows.CloseHandle(job)
		return 0
	}

	return uintptr(job)
}

// addProcessToJobObject は指定 PID のプロセスを Job Object に割り当てる。
func addProcessToJobObject(jobHandle uintptr, pid int) {
	if jobHandle == 0 {
		return
	}
	ph, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(pid))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] OpenProcess 失敗 (pid=%d): %v\n", pid, err)
		return
	}
	defer windows.CloseHandle(ph)

	if err := windows.AssignProcessToJobObject(windows.Handle(jobHandle), ph); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] AssignProcessToJobObject 失敗 (pid=%d): %v\n", pid, err)
	}
}
