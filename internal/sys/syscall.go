package sys

import (
	"runtime"
	"syscall"
	"unsafe"
)

func IoUringSetup(entries uint32, p *IoUringParams) (int, syscall.Errno) {
	res, _, err := syscall.RawSyscall(__NR_io_uring_setup, uintptr(entries), uintptr(unsafe.Pointer(p)), 0)
	runtime.KeepAlive(p)
	return int(res), err
}

func IoUringEnter(fd int, toSubmit uint32, minComplete uint32, flags uint32, args unsafe.Pointer, argsz int) (int, syscall.Errno) {
	res, _, err := syscall.Syscall6(__NR_io_uring_enter, uintptr(fd), uintptr(toSubmit), uintptr(minComplete), uintptr(flags), uintptr(args), uintptr(argsz))
	runtime.KeepAlive(args)
	return int(res), err
}

func IoUringEnterFast(fd int, toSubmit uint32, minComplete uint32, flags uint32, args uintptr, narg int) (int, syscall.Errno) {
	res, _, err := syscall.RawSyscall6(__NR_io_uring_enter, uintptr(fd), uintptr(toSubmit), uintptr(minComplete), uintptr(flags), args, uintptr(narg))
	return int(res), err
}

func IoUringRegister(fd int, opcode uint, arg unsafe.Pointer, nargs uint) (int, syscall.Errno) {
	res, _, err := syscall.RawSyscall6(__NR_io_uring_register, uintptr(fd), uintptr(opcode), uintptr(arg), uintptr(nargs), 0, 0)
	runtime.KeepAlive(arg)
	return int(res), err
}

func Mmap(addr uintptr, length int, prot int, flags int, fd int, offset int) (uintptr, syscall.Errno) {
	p, _, err := syscall.RawSyscall6(syscall.SYS_MMAP, addr, uintptr(length), uintptr(prot), uintptr(flags), uintptr(fd), uintptr(offset))
	return p, err
}

func Munmap(addr uintptr, length int) syscall.Errno {
	_, _, err := syscall.RawSyscall(syscall.SYS_MUNMAP, addr, uintptr(length), 0)
	return err
}
