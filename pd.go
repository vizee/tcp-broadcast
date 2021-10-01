package main

import (
	"unsafe"
)

const (
	tagListenerAccept uint32 = iota
	tagConnSend
	tagConnRecv
)

const (
	flagPolling uint32 = 1 << iota
	flagCancel
)

type PollDesc struct {
	tag   uint32
	flags uint32
	data  uint64
}

func (p *PollDesc) ptr() uint64 {
	return uint64(uintptr(unsafe.Pointer(p)))
}

func (p *PollDesc) polling() bool {
	return p.flags&flagPolling != 0
}

func (p *PollDesc) markPolling() bool {
	if p.flags&(flagCancel|flagPolling) == 0 {
		p.flags |= flagPolling
		return true
	} else {
		return false
	}
}

var completeFuncs = [...]func(*PollDesc, int32) error{
	tagListenerAccept: completeAccept,
	tagConnSend:       completeSend,
	tagConnRecv:       completeRecv,
}

func completeAccept(pd *PollDesc, res int32) error {
	ln := *(**TcpListener)(unsafe.Pointer(&pd.data))
	return ln.accept(int(res))
}

func completeSend(pd *PollDesc, res int32) error {
	conn := *(**TcpConn)(unsafe.Pointer(&pd.data))
	return conn.completeSend(int(res))
}

func completeRecv(pd *PollDesc, res int32) error {
	conn := *(**TcpConn)(unsafe.Pointer(&pd.data))
	return conn.completeRecv(int(res))
}

func completeIo(data uint64, res int32, _ uint32) error {
	pd := *(**PollDesc)(unsafe.Pointer(&data))
	pd.flags ^= flagPolling

	if pd.flags&flagCancel != 0 {
		completeCancel(pd)
		return nil
	}
	return completeFuncs[pd.tag](pd, res)
}
