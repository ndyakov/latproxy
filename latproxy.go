// latproxy is a TCP delay proxy for latency-sweep benchmarking on loopback.
// It forwards listen -> target adding a fixed one-way delay of rtt/2 in EACH
// direction, so a request/reply round trip pays ~rtt. Delays are applied via
// per-direction delivery queues (read chunks are timestamped and written after
// their deadline), so bandwidth is not serialized by the delay — many chunks
// can be "in flight" concurrently, like a real long-fat pipe.
package main

import (
	"flag"
	"io"
	"log"
	"net"
	"time"
)

type chunk struct {
	data []byte
	due  time.Time
}

func pipe(dst, src net.Conn, delay time.Duration, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	q := make(chan chunk, 4096)
	go func() {
		for c := range q {
			if d := time.Until(c.due); d > 0 {
				time.Sleep(d)
			}
			if _, err := dst.Write(c.data); err != nil {
				// Drain the queue so the reader can exit on its own error.
				for range q {
				}
				return
			}
		}
		if tc, ok := dst.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()
	for {
		buf := make([]byte, 32*1024)
		n, err := src.Read(buf)
		if n > 0 {
			q <- chunk{data: buf[:n], due: time.Now().Add(delay)}
		}
		if err != nil {
			close(q)
			if err != io.EOF {
				return
			}
			return
		}
	}
}

func main() {
	listen := flag.String("listen", "127.0.0.1:6395", "listen address")
	target := flag.String("target", "127.0.0.1:6379", "target address")
	rtt := flag.Duration("rtt", 10*time.Millisecond, "round-trip time to add (half per direction)")
	flag.Parse()

	oneWay := *rtt / 2
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	log.Printf("latproxy %s -> %s rtt=%s", *listen, *target, *rtt)
	for {
		cli, err := ln.Accept()
		if err != nil {
			log.Fatalf("accept: %v", err)
		}
		go func(cli net.Conn) {
			srv, err := net.Dial("tcp", *target)
			if err != nil {
				_ = cli.Close()
				return
			}
			_ = cli.(*net.TCPConn).SetNoDelay(true)
			_ = srv.(*net.TCPConn).SetNoDelay(true)
			done := make(chan struct{}, 2)
			go pipe(srv, cli, oneWay, done)
			go pipe(cli, srv, oneWay, done)
			<-done
			<-done
			_ = cli.Close()
			_ = srv.Close()
		}(cli)
	}
}
