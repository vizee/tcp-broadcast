package yaur

import (
	"syscall"
	"tcp-broadcast/internal/sys"
	"unsafe"
)

func SubmitAccept(poll *IoPoll, data uint64, fd int, sa *syscall.RawSockaddrAny, salen *int) error {
	sqe, err := poll.GetSqe()
	if err != nil {
		return err
	}

	*sqe = sys.IoUringSqe{
		Opcode:    sys.IORING_OP_ACCEPT,
		Fd:        int32(fd),
		Off:       uint64(uintptr(unsafe.Pointer(salen))),
		Addr:      uint64(uintptr(unsafe.Pointer(sa))),
		User_data: data,
	}
	return nil
}

func SubmitAsyncCancel(poll *IoPoll, data uint64, key uint64) error {
	sqe, err := poll.GetSqe()
	if err != nil {
		return err
	}

	*sqe = sys.IoUringSqe{
		Opcode:    sys.IORING_OP_ASYNC_CANCEL,
		Addr:      key,
		User_data: data,
	}
	return nil
}

func SubmitSend(poll *IoPoll, data uint64, fd int, off int, buf uintptr, size int) error {
	sqe, err := poll.GetSqe()
	if err != nil {
		return err
	}

	*sqe = sys.IoUringSqe{
		Opcode:    sys.IORING_OP_SEND,
		Fd:        int32(fd),
		Off:       uint64(off),
		Addr:      uint64(buf),
		Len:       uint32(size),
		User_data: data,
	}
	return nil
}

func SubmitRecv(poll *IoPoll, data uint64, fd int, off int, buf uintptr, size int) error {
	sqe, err := poll.GetSqe()
	if err != nil {
		return err
	}

	*sqe = sys.IoUringSqe{
		Opcode:    sys.IORING_OP_RECV,
		Fd:        int32(fd),
		Off:       uint64(off),
		Addr:      uint64(buf),
		Len:       uint32(size),
		User_data: data,
	}
	return nil
}
