package yaur

import (
	"fmt"
	"sync/atomic"
	"syscall"
	"tcp-broadcast/internal/sys"
	"unsafe"
)

const maxRingSize = 0xffffffff

type mappedSq struct {
	head        *uint32
	tail        *uint32
	ringMask    *uint32
	ringEntries *uint32
	flags       *uint32
	dropped     *uint32
	array       *[maxRingSize]uint32
	sqes        *[maxRingSize]sys.IoUringSqe
}

type mappedCq struct {
	head        *uint32
	tail        *uint32
	ringMask    *uint32
	ringEntries *uint32
	overflow    *uint32
	cqes        *[maxRingSize]sys.IoUringCqe
}

type ring struct {
	head uint32
	tail uint32
}

type IoPoll struct {
	ringFd     int
	sq         mappedSq
	cq         mappedCq
	params     sys.IoUringParams
	completeIo func(uint64, int32, uint32) error

	cachedSq ring
}

func (p *IoPoll) checkFeatures() error {
	requiredFeatures := sys.IORING_FEAT_SINGLE_MMAP | sys.IORING_FEAT_NODROP | sys.IORING_FEAT_FAST_POLL
	if p.params.Features&uint32(requiredFeatures) != uint32(requiredFeatures) {
		return fmt.Errorf("features not matched")
	}

	maxOps := sys.IORING_OP_RECV

	var probe sys.IoUringProbe
	_, eno := sys.IoUringRegister(p.ringFd, sys.IORING_REGISTER_PROBE, unsafe.Pointer(&probe), 0)
	if eno != 0 {
		return eno
	}
	if maxOps > int(probe.Last_op) {
		return fmt.Errorf("op not supported")
	}

	return nil
}

func (p *IoPoll) unmapUring() {
	if p.sq.sqes != nil {
		sqesSize := *p.sq.ringEntries * uint32(unsafe.Sizeof(sys.IoUringSqe{}))
		sys.Munmap(uintptr(unsafe.Pointer(p.sq.sqes)), int(sqesSize))
		p.sq.sqes = nil
	}
	if p.sq.head != nil {
		sqOff := &p.params.Sq_off
		ringSize := sqOff.Array + p.params.Sq_entries*uint32(unsafe.Sizeof(uint32(0)))
		ringPtr := uintptr(unsafe.Pointer(p.sq.head)) - uintptr(sqOff.Head)
		sys.Munmap(ringPtr, int(ringSize))
		p.sq = mappedSq{}
		p.cq = mappedCq{}
	}
}

func (p *IoPoll) mapUring() error {
	sqOff := &p.params.Sq_off
	ringSize := sqOff.Array + p.params.Sq_entries*uint32(unsafe.Sizeof(uint32(0)))

	base, eno := sys.Mmap(0, int(ringSize), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED|syscall.MAP_POPULATE, p.ringFd, sys.IORING_OFF_SQ_RING)
	if eno != 0 {
		return eno
	}

	ringPtr := *(*unsafe.Pointer)(unsafe.Pointer(&base))
	sq := &p.sq
	sq.head = (*uint32)(unsafe.Add(ringPtr, int(sqOff.Head)))
	sq.tail = (*uint32)(unsafe.Add(ringPtr, int(sqOff.Tail)))
	sq.ringMask = (*uint32)(unsafe.Add(ringPtr, int(sqOff.Mask)))
	sq.ringEntries = (*uint32)(unsafe.Add(ringPtr, int(sqOff.Entries)))
	sq.flags = (*uint32)(unsafe.Add(ringPtr, int(sqOff.Flags)))
	sq.dropped = (*uint32)(unsafe.Add(ringPtr, int(sqOff.Dropped)))
	sq.array = (*[maxRingSize]uint32)(unsafe.Add(ringPtr, int(sqOff.Array)))

	sqesSize := p.params.Sq_entries * uint32(unsafe.Sizeof(sys.IoUringSqe{}))
	sqes, eno := sys.Mmap(0, int(sqesSize), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED|syscall.MAP_POPULATE, p.ringFd, sys.IORING_OFF_SQES)
	if eno != 0 {
		return eno
	}
	sq.sqes = (*[maxRingSize]sys.IoUringSqe)(*(*unsafe.Pointer)(unsafe.Pointer(&sqes)))

	cqOff := &p.params.Cq_off
	cq := &p.cq
	cq.head = (*uint32)(unsafe.Add(ringPtr, int(cqOff.Head)))
	cq.tail = (*uint32)(unsafe.Add(ringPtr, int(cqOff.Tail)))
	cq.ringMask = (*uint32)(unsafe.Add(ringPtr, int(cqOff.Mask)))
	cq.ringEntries = (*uint32)(unsafe.Add(ringPtr, int(cqOff.Entries)))
	cq.overflow = (*uint32)(unsafe.Add(ringPtr, int(cqOff.Overflow)))
	cq.cqes = (*[maxRingSize]sys.IoUringCqe)(unsafe.Add(ringPtr, int(cqOff.Cqes)))

	return nil
}

func (p *IoPoll) flushCachedSq() int {
	tail := *p.sq.tail
	mask := *p.sq.ringMask
	toSubmit := p.cachedSq.tail - p.cachedSq.head
	for p.cachedSq.head != p.cachedSq.tail {
		p.sq.array[tail&mask] = p.cachedSq.head & mask
		p.cachedSq.head++
		tail++
	}
	if toSubmit > 0 {
		atomic.StoreUint32(p.sq.tail, tail)
	}
	return int(toSubmit)
}

func (p *IoPoll) submitCachedSq() error {
	toSubmit := p.flushCachedSq()
	if toSubmit > 0 {
		_, eno := sys.IoUringEnterFast(p.ringFd, uint32(toSubmit), 0, 0, 0, 0)
		if eno != 0 {
			return eno
		}
	}
	return nil
}

func (p *IoPoll) GetSqe() (*sys.IoUringSqe, error) {
	head := atomic.LoadUint32(p.sq.head)
	tail := p.cachedSq.tail
	if tail-head >= *p.sq.ringEntries {
		err := p.submitCachedSq()
		if err != nil {
			return nil, err
		}
	}
	p.cachedSq.tail = tail + 1
	return &p.sq.sqes[tail&*p.sq.ringMask], nil
}

func (p *IoPoll) consumeCq() error {
	var err error
	head := *p.cq.head
	for isReady(head, p.cq.tail) {
		cqe := &p.cq.cqes[head&*p.cq.ringMask]
		head++
		err = p.completeIo(cqe.Data, cqe.Res, cqe.Flags)
		if err != nil {
			break
		}
	}
	atomic.StoreUint32(p.cq.head, head)
	return err
}

func (p *IoPoll) Close() error {
	p.unmapUring()
	return syscall.Close(p.ringFd)
}

func (p *IoPoll) TryPoll() (bool, error) {
	err := p.consumeCq()
	if err != nil {
		return false, err
	}
	err = p.submitCachedSq()
	if err != nil {
		return false, err
	}
	return isReady(*p.cq.head, p.cq.tail), nil
}

func (p *IoPoll) PollWait() error {
	p.consumeCq()
	toSubmit := p.flushCachedSq()
	minComplete := uint32(1)
	if isReady(*p.cq.head, p.cq.tail) {
		minComplete = 0
	}
	for {
		_, eno := sys.IoUringEnter(p.ringFd, uint32(toSubmit), minComplete, sys.IORING_ENTER_GETEVENTS, nil, 0)
		if eno != 0 {
			if eno == syscall.EINTR {
				continue
			}
			return eno
		}
		return nil
	}
}

func NewIoPoll(setupFlags uint32, queueDepth int, completeIo func(uint64, int32, uint32) error) (*IoPoll, error) {
	poll := &IoPoll{
		completeIo: completeIo,
	}
	poll.params.Flags = uint32(setupFlags)
	fd, eno := sys.IoUringSetup(uint32(queueDepth), &poll.params)
	if eno != 0 {
		return nil, eno
	}
	poll.ringFd = fd
	err := poll.checkFeatures()
	if err != nil {
		_ = poll.Close()
		return nil, err
	}
	err = poll.mapUring()
	if err != nil {
		_ = poll.Close()
		return nil, err
	}
	return poll, nil
}

func isReady(head uint32, tail *uint32) bool {
	return head != atomic.LoadUint32(tail)
}
