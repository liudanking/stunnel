package main

import (
	"crypto/tls"
	"net"
	"sync/atomic"
	"time"

	log "github.com/liudanking/log4go"
)

type RemoteServer struct {
	laddr       string
	raddr       string
	streamID    uint64
	streamCount int32
}

func NewRemoteServer(laddr, raddr string) (*RemoteServer, error) {
	rs := &RemoteServer{
		laddr:       laddr,
		raddr:       raddr,
		streamCount: 0,
	}
	return rs, nil
}

func (rs *RemoteServer) Serve(certFile, keyFile string) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Error("load cert/key error:%v", err)
		return
	}
	config := tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	l, err := tls.Listen("tcp", rs.laddr, &config)
	if err != nil {
		log.Error("net.ListenTCP(%s) error:%v", rs.laddr, err)
		return
	}

	for {
		stream, err := l.Accept()
		if err != nil {
			log.Error("listenner accept connection error:%v", err)
			continue
		}
		go rs.serveStream(stream)
	}
}

func (rs *RemoteServer) serveStream(stream net.Conn) {
	defer stream.Close()
	start := time.Now()
	conn, err := net.Dial("tcp", rs.raddr)
	if err != nil {
		log.Error("connect to remote error:%v", err)
		return
	}
	defer conn.Close()

	atomic.AddInt32(&rs.streamCount, 1)
	streamID := atomic.AddUint64(&rs.streamID, 1)
	readChan := make(chan int64, 1)
	writeChan := make(chan int64, 1)
	var readBytes int64
	var writeBytes int64
	go pipe(stream, conn, readChan)
	go pipe(conn, stream, writeChan)
	for i := 0; i < 2; i++ {
		select {
		case readBytes = <-readChan:
			stream.Close()
			log.Debug("[#%d] read %d bytes", streamID, readBytes)
		case writeBytes = <-writeChan:
			// DON'T call conn.Close, it will trigger an error in pipe.
			// Just close stream, let the conn copy finished normally
			log.Debug("[#%d] write %d bytes", streamID, writeBytes)
		}
	}
	log.Info("[#%d] r:%d w:%d t:%v c:%d",
		streamID, readBytes, writeBytes, time.Now().Sub(start), atomic.LoadInt32(&rs.streamCount))
	atomic.AddInt32(&rs.streamCount, -1)
}
