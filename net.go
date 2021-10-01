package main

import (
	"net"
	"strconv"
	"syscall"
	"tcp-broadcast/internal/yaur"
	"unsafe"
)

type ConnIoFunc func(c *TcpConn, res int) error

type AcceptFunc func(int) error

type TcpConn struct {
	rpd     PollDesc
	wpd     PollDesc
	fd      int
	inbuf   []byte
	inlen   int
	outbuf  []byte
	sendbuf []byte

	recv ConnIoFunc
	send ConnIoFunc
}

func (c *TcpConn) completeRecv(res int) error {
	if res < 0 {
		return c.recv(c, res)
	}
	if res == 0 {
		return c.recv(c, 0)
	}

	c.inlen += res
	return c.recv(c, c.inlen)
}

func (c *TcpConn) completeSend(res int) error {
	if res < 0 {
		return c.send(c, res)
	}
	if len(c.outbuf) == res {
		c.outbuf = nil
	} else {
		c.outbuf = c.outbuf[res:]
	}
	sendIdle := len(c.outbuf) == 0
	err := c.asyncSend(nil)
	if err != nil {
		return err
	}
	if sendIdle {
		return c.send(c, 0)
	}
	return nil
}

func (c *TcpConn) asyncRecv(n int) error {
	if !c.rpd.markPolling() {
		return nil
	}
	if n == 0 {
		return syscall.EINVAL
	}
	sz := c.inlen + n
	switch {
	case len(c.inbuf) == sz:
	case sz > cap(c.inbuf):
		inbuf := make([]byte, sz)
		copy(inbuf, c.inbuf[:c.inlen])
		c.inbuf = inbuf
	default:
		c.inbuf = c.inbuf[:sz]
	}

	return yaur.SubmitRecv(app.poll, c.rpd.ptr(), c.fd, c.inlen, uintptr(unsafe.Pointer(&c.inbuf[0])), n)
}

func (c *TcpConn) asyncSend(buf []byte) error {
	if !c.wpd.markPolling() {
		c.sendbuf = append(c.sendbuf, buf...)
		return nil
	}

	if len(c.outbuf) == 0 {
		c.outbuf = c.sendbuf
		c.sendbuf = nil
	}
	if len(buf) > 0 {
		c.outbuf = append(c.outbuf, buf...)
	}
	if len(c.outbuf) == 0 {
		return nil
	}

	return yaur.SubmitSend(app.poll, c.wpd.ptr(), c.fd, 0, uintptr(unsafe.Pointer(&c.outbuf[0])), len(c.outbuf))
}

func newTcpConn(fd int, recv ConnIoFunc, send ConnIoFunc) *TcpConn {
	c := &TcpConn{
		fd:   fd,
		recv: recv,
		send: send,
	}
	c.rpd.tag = tagConnRecv
	c.rpd.data = uint64(uintptr(unsafe.Pointer(c)))
	c.wpd.tag = tagConnSend
	c.wpd.data = uint64(uintptr(unsafe.Pointer(c)))
	return c
}

type TcpListener struct {
	pd     PollDesc
	fd     int
	accept AcceptFunc
}

func (l *TcpListener) asyncAccept() error {
	if !l.pd.markPolling() {
		return nil
	}
	return yaur.SubmitAccept(app.poll, l.pd.ptr(), l.fd, nil, nil)
}

func listenTcp(network string, address string, backlog int, accept AcceptFunc, opts ...func(fd int) error) (*TcpListener, error) {
	na, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, err
	}
	ip := na.IP
	if ip == nil {
		ip = net.IPv6zero
	} else {
		ip = ip.To4()
		if ip == nil {
			ip = na.IP
		}
	}
	var (
		af int
		sa syscall.Sockaddr
	)
	if len(ip) == net.IPv4len {
		sa4 := &syscall.SockaddrInet4{
			Port: na.Port,
		}
		copy(sa4.Addr[:], ip)
		af = syscall.AF_INET
		sa = sa4
	} else if len(ip) == net.IPv6len {
		sa6 := &syscall.SockaddrInet6{
			Port: na.Port,
		}
		copy(sa6.Addr[:], ip)
		if na.Zone != "" {
			nif, err := net.InterfaceByName(na.Zone)
			if err == nil {
				sa6.ZoneId = uint32(nif.Index)
			} else {
				n, _ := strconv.Atoi(na.Zone)
				sa6.ZoneId = uint32(n)
			}
		}
		af = syscall.AF_INET6
		sa = sa6
	}

	fd, err := syscall.Socket(af, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, err
	}
	err = func() error {
		for _, opt := range opts {
			if err := opt(fd); err != nil {
				return err
			}
		}
		if err := syscall.Bind(fd, sa); err != nil {
			return err
		}
		return syscall.Listen(fd, backlog)
	}()
	if err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}

	ln := &TcpListener{
		fd:     fd,
		accept: accept,
	}
	ln.pd.tag = tagListenerAccept
	ln.pd.data = uint64(uintptr(unsafe.Pointer(ln)))
	return ln, nil
}
