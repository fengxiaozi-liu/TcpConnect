package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"tcpconnect/internal/domain/fix"
	"time"
)

func main() {
	port := flag.Int("port", 9878, "loopback FIX port")
	flag.Parse()
	l, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if e != nil {
		log.Fatal(e)
	}
	defer l.Close()
	log.Printf("FIX mock: %s (Sender=Fix, Target=FixClient)", l.Addr())
	for {
		c, e := l.Accept()
		if e != nil {
			log.Fatal(e)
		}
		go serve(c)
	}
}
func serve(c net.Conn) {
	defer c.Close()
	seq := 1
	version := ""
	buffer := []byte{}
	chunk := make([]byte, 32768)
	send := func(typ string, body []fix.Pair) error {
		_ = c.SetWriteDeadline(time.Now().Add(2 * time.Second))
		raw := fix.Encode(version, typ, "Fix", "FixClient", seq, body, time.Now())
		seq++
		_, e := io.Copy(c, bytes.NewReader(raw))
		return e
	}
	for {
		n, e := c.Read(chunk)
		if e != nil {
			return
		}
		buffer = append(buffer, chunk[:n]...)
		for len(buffer) > 0 {
			size, e := fix.FrameLength(buffer)
			if e != nil {
				log.Printf("mock: invalid frame: %v", e)
				return
			}
			if size == 0 {
				break
			}
			ps, e := fix.Pairs(buffer[:size])
			buffer = buffer[size:]
			if e != nil {
				return
			}
			if version == "" {
				version = fix.Get(ps, 8)
			} else if fix.Get(ps, 8) != version {
				return
			}
			switch fix.Get(ps, 35) {
			case "A":
				seq = 1
				body := []fix.Pair{{Tag: 98, Value: "0"}, {Tag: 108, Value: "30"}}
				if version != "FIX.4.0" {
					body = append(body, fix.Pair{Tag: 141, Value: "Y"})
				}
				e = send("A", body)
			case "0":
				e = send("0", nil)
			case "1":
				e = send("0", []fix.Pair{{Tag: 112, Value: fix.Get(ps, 112)}})
			case "5":
				_ = send("5", nil)
				return
			case "D":
				qty := fix.Get(ps, 38)
				if qty == "" {
					qty = "0"
				}
				body := []fix.Pair{{Tag: 37, Value: "MOCK-" + strconv.FormatInt(time.Now().UnixMilli(), 10)}, {Tag: 11, Value: fix.Get(ps, 11)}, {Tag: 17, Value: "EXEC-" + strconv.Itoa(seq)}, {Tag: 150, Value: "0"}, {Tag: 39, Value: "0"}, {Tag: 55, Value: fix.Get(ps, 55)}, {Tag: 54, Value: fix.Get(ps, 54)}, {Tag: 151, Value: qty}, {Tag: 14, Value: "0"}, {Tag: 6, Value: "0"}, {Tag: 58, Value: "Local mock: order accepted"}}
				e = send("8", body)
			default:
				e = send("j", []fix.Pair{{Tag: 372, Value: fix.Get(ps, 35)}, {Tag: 380, Value: "3"}, {Tag: 58, Value: "Message type not implemented by local mock"}})
			}
			if e != nil {
				return
			}
		}
	}
}
