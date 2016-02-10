package main

import (
	"net"
	"sync/atomic"
	"time"

	log "github.com/liudanking/log4go"
)

type LocalServer struct {
	laddr       string
	raddr       string
	streamID    uint64
	streamCount int32
}

func NewLocalServer(laddr, raddr string, tunnelCount int) (*LocalServer, error) {
	ls := &LocalServer{
		laddr:       laddr,
		raddr:       raddr,
		streamCount: 0,
	}
	return ls, nil
}

func (ls *LocalServer) Serve() {
	l, err := net.Listen("tcp", ls.laddr)
	if err != nil {
		log.Error("listen [%s] error:%v", ls.laddr, err)
		return
	}

	for {
		c, err := l.Accept()
		if err != nil {
			log.Error("accept connection error:%v", err)
			continue
		}
		go ls.transport(c)
	}
}

func (ls *LocalServer) transport(conn net.Conn) {
	defer conn.Close()
	start := time.Now()
	stream, err := ls.openStream()
	if err != nil {
		log.Error("open stream for %s error:%v", conn.RemoteAddr().String(), err)
		return
	}
	defer stream.Close()

	atomic.AddInt32(&ls.streamCount, 1)
	streamID := atomic.AddUint64(&ls.streamID, 1)
	readChan := make(chan int64, 1)
	writeChan := make(chan int64, 1)
	var readBytes int64
	var writeBytes int64
	go pipe(conn, stream, readChan)
	go pipe(stream, conn, writeChan)
	for i := 0; i < 2; i++ {
		select {
		case readBytes = <-readChan:
			// DON'T call conn.Close, it will trigger an error in pipe.
			// Just close stream, let the conn copy finished normally
			log.Debug("[#%d] read %d bytes", streamID, readBytes)
		case writeBytes = <-writeChan:
			stream.Close()
			log.Debug("[#%d] write %d bytes", streamID, writeBytes)
		}
	}
	log.Info("[#%d] r:%d w:%d t:%v c:%d",
		streamID, readBytes, writeBytes, time.Now().Sub(start), atomic.LoadInt32(&ls.streamCount))
	atomic.AddInt32(&ls.streamCount, -1)
}

func (ls *LocalServer) openStream() (stream net.Conn, err error) {
	// TODO: try to dial multiple remote addresses when meet an error
	return dialTLS(ls.raddr)
}
