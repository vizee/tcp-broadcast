package main

import (
	"log"
	"runtime"
	"syscall"
	"tcp-broadcast/internal/yaur"
)

var app struct {
	poll    *yaur.IoPoll
	cancels map[*PollDesc]struct{}
	ln      *TcpListener
	conns   map[*TcpConn]struct{}
}

func closeConn(c *TcpConn) error {
	delete(app.conns, c)
	deferClose := false
	if c.rpd.polling() {
		err := cancelIo(&c.rpd)
		if err != nil {
			return err
		}
		deferClose = true
	}
	if c.wpd.polling() {
		err := cancelIo(&c.wpd)
		if err != nil {
			return err
		}
		deferClose = true
	}
	if deferClose {
		runtime.SetFinalizer(c, func(c *TcpConn) { syscall.Close(c.fd) })
	} else {
		syscall.Close(c.fd)
	}
	return nil
}

func connRecv(c *TcpConn, res int) error {
	if res < 0 {
		log.Printf("recv error: %v", syscall.Errno(-res))
		return closeConn(c)
	} else if res == 0 {
		// EOF
		return closeConn(c)
	}

	data := append(make([]byte, 0, res), c.inbuf[:res]...)
	for conn := range app.conns {
		if conn == c {
			continue
		}
		err := conn.asyncSend(data)
		if err != nil {
			return err
		}
	}

	c.inlen = 0
	return c.asyncRecv(128)
}

func connSend(c *TcpConn, res int) error {
	if res < 0 {
		log.Printf("send error: %v", syscall.Errno(-res))
		return closeConn(c)
	}
	return nil
}

func acceptConn(res int) error {
	if res < 0 {
		log.Printf("accept: %v", syscall.Errno(-res))
	} else {
		conn := newTcpConn(res, connRecv, connSend)
		app.conns[conn] = struct{}{}
		err := conn.asyncRecv(128)
		if err != nil {
			return err
		}
	}
	return app.ln.asyncAccept()
}

func cancelIo(pd *PollDesc) error {
	if !pd.polling() {
		return nil
	}
	pd.flags |= flagCancel
	cpd := &PollDesc{
		tag:   tagCancelIo,
		flags: flagPolling,
		data:  pd.ptr(),
	}
	app.cancels[pd] = struct{}{}
	app.cancels[cpd] = struct{}{}

	return yaur.SubmitAsyncCancel(app.poll, cpd.ptr(), pd.ptr())
}

func pollLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for {
		ready, err := app.poll.TryPoll()
		if err != nil {
			log.Fatalf("TryPoll: %v", err)
		}
		if ready {
			continue
		}
		err = app.poll.PollWait()
		if err != nil {
			log.Fatalf("PollWait: %v", err)
		}
	}
}

func shutdown() {
	app.poll.Close()
}

func setup() error {
	app.cancels = make(map[*PollDesc]struct{})
	app.conns = make(map[*TcpConn]struct{})

	poll, err := yaur.NewIoPoll(0, 256, completeIo)
	if err != nil {
		return err
	}
	app.poll = poll
	return nil
}

func main() {
	err := setup()
	if err != nil {
		log.Fatalf("setup: %v", err)
	}

	ln, err := listenTcp("tcp4", ":7669", syscall.SOMAXCONN, acceptConn, func(fd int) error {
		return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	})
	if err != nil {
		log.Fatalf("listenTcp: %v", err)
	}
	app.ln = ln
	err = ln.asyncAccept()
	if err != nil {
		log.Fatalf("asyncAccept: %v", err)
	}

	pollLoop()
	shutdown()
}
