//go:build ignore

package sys

/*
#include <sys/syscall.h>
#ifdef INC_SYS_URING
#include <sys/io_uring.h>
#else
#include "include/io_uring.h"
#endif
*/
import "C"

// sys/syscall.h

const __NR_io_uring_setup = C.__NR_io_uring_setup
const __NR_io_uring_enter = C.__NR_io_uring_enter
const __NR_io_uring_register = C.__NR_io_uring_register

// sys/io_uring.h

const (
	IORING_OP_ACCEPT       = C.IORING_OP_ACCEPT
	IORING_OP_ASYNC_CANCEL = C.IORING_OP_ASYNC_CANCEL
	IORING_OP_CLOSE        = C.IORING_OP_CLOSE
	IORING_OP_SEND         = C.IORING_OP_SEND
	IORING_OP_RECV         = C.IORING_OP_RECV
)

const IORING_OFF_SQ_RING = C.IORING_OFF_SQ_RING
const IORING_OFF_SQES = C.IORING_OFF_SQES

const IORING_ENTER_GETEVENTS = C.IORING_ENTER_GETEVENTS

const IORING_FEAT_SINGLE_MMAP = C.IORING_FEAT_SINGLE_MMAP
const IORING_FEAT_NODROP = C.IORING_FEAT_NODROP
const IORING_FEAT_FAST_POLL = C.IORING_FEAT_FAST_POLL

const IORING_REGISTER_PROBE = C.IORING_REGISTER_PROBE

type IoUringSqe C.struct_io_uring_sqe

type IoUringCqe C.struct_io_uring_cqe

type IoSqringOffsets C.struct_io_sqring_offsets

type IoCqringOffsets C.struct_io_cqring_offsets

type IoUringParams C.struct_io_uring_params

type IoUringProbeOp C.struct_io_uring_probe_op

type IoUringProbe C.struct_io_uring_probe
