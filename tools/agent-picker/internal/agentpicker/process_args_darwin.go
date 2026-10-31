package agentpicker

import (
	"syscall"
	"unsafe"
)

// KERN_PROCARGS2 returns argc, the executable path, padding, argv, then env.
// Keep the original argument boundaries; ps args cannot distinguish a prompt
// containing spaces or flags from several actual command-line arguments.
func processArgs(pid int) ([]string, error) {
	size, err := syscall.SysctlUint32("kern.argmax")
	if err != nil {
		return nil, err
	}
	if size < 4 || size > 16*1024*1024 {
		return nil, syscall.EINVAL
	}
	buf := make([]byte, size)
	n := uintptr(len(buf))
	mib := [3]int32{1, 49, int32(pid)} // CTL_KERN, KERN_PROCARGS2, pid
	_, _, errno := syscall.Syscall6(syscall.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])), uintptr(len(mib)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)), 0, 0)
	if errno != 0 {
		return nil, errno
	}
	if n > uintptr(len(buf)) {
		return nil, syscall.EINVAL
	}
	return decodeProcessArgs(buf[:n])
}
