package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
)

func main() {
	port := flag.Int("port", 9879, "loopback TCP port")
	flag.Parse()
	l, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if e != nil {
		log.Fatal(e)
	}
	defer l.Close()
	log.Printf("Echo: %s", l.Addr())
	for {
		c, e := l.Accept()
		if e != nil {
			log.Fatal(e)
		}
		go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
	}
}
