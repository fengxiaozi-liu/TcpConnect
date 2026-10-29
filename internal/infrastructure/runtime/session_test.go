package runtime

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/dop251/goja"
	"net"
	"os"
	"strconv"
	"strings"
	"tcpconnect/internal/domain"
	"tcpconnect/internal/domain/fix"
	"testing"
	"time"
)

func listener(t *testing.T) (net.Listener, int) {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { l.Close() })
	return l, l.Addr().(*net.TCPAddr).Port
}

func TestFIXReplayAndGapFill(t *testing.T) {
	client, peer := net.Pipe()
	defer client.Close()
	defer peer.Close()
	s := &Session{conn: client, connected: true, seq: 4, expected: 1, pending: map[int][]byte{}, journal: map[int][]byte{}, opts: domain.SessionOptions{Service: domain.Service{Sender: "CLIENT", Target: "SERVER"}, Emit: func(domain.Event) {}}}
	for i, typ := range []string{"A", "D", "0"} {
		s.remember(i+1, fix.Encode(typ, "CLIENT", "SERVER", i+1, []fix.Pair{{Tag: 11, Value: "ORDER"}}, time.Now()))
	}
	done := make(chan error, 1)
	go func() {
		done <- s.receiveFIX(fix.Encode("2", "SERVER", "CLIENT", 1, []fix.Pair{{Tag: 7, Value: "1"}, {Tag: 16, Value: "3"}}, time.Now()))
	}()
	for i, typ := range []string{"4", "D", "4"} {
		raw, err := readFIX(peer)
		if err != nil {
			t.Fatal(err)
		}
		ps, err := fix.Pairs(raw)
		if err != nil {
			t.Fatal(err)
		}
		if fix.Get(ps, 35) != typ || fix.Get(ps, 34) != strconv.Itoa(i+1) || fix.Get(ps, 43) != "Y" {
			t.Fatalf("incorrect replay: %v", ps)
		}
		if typ == "D" && fix.Get(ps, 122) == "" {
			t.Fatal("missing OrigSendingTime")
		}
		if typ == "4" && fix.Get(ps, 36) != strconv.Itoa(i+2) {
			t.Fatal("wrong gap fill")
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if s.seq != 4 || s.expected != 2 {
		t.Fatal("replay advanced live sending sequence")
	}
}

func TestDisplayErrorDoesNotSkipFIXAndPostScript(t *testing.T) {
	events := []domain.Event{}
	s := &Session{dictionary: &fix.Dictionary{}, vm: goja.New(), functions: map[string]domain.Function{"broken": {Code: `throw new Error("display failed")`}}, vars: map[string]any{}, tasks: map[string]*task{}, pending: map[int][]byte{}, expected: 1, active: domain.Request{ResponseFunctionID: "broken", PostScript: `vars.set("post",true)`}, opts: domain.SessionOptions{Service: domain.Service{Sender: "CLIENT", Target: "SERVER"}, Emit: func(e domain.Event) { events = append(events, e) }}}
	if err := s.initVM(); err != nil {
		t.Fatal(err)
	}
	raw := fix.Encode("0", "SERVER", "CLIENT", 1, nil, time.Now())
	if err := s.receive(raw); err != nil {
		t.Fatal(err)
	}
	if s.expected != 2 || s.vars["post"] != true {
		t.Fatal("display failure skipped protocol or post script")
	}
	found := false
	for _, e := range events {
		if e.Kind == "error" {
			found = true
		}
	}
	if !found {
		t.Fatal("display error was not visible")
	}
}

func TestFIXIdleHeartbeatUsesCurrentConnection(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	s := &Session{conn: client, connected: true, loggedOn: true, seq: 1, tasks: map[string]*task{}, journal: map[int][]byte{}, lastRead: time.Now(), lastWrite: time.Now().Add(-2 * time.Second), opts: domain.SessionOptions{Service: domain.Service{Sender: "CLIENT", Target: "SERVER", Heartbeat: 1}, Emit: func(domain.Event) {}}}
	defer s.Close()
	stopped := make(chan struct{})
	go func() { s.heartbeatLoop(); close(stopped) }()
	raw, err := readFIX(peer)
	if err != nil {
		t.Fatal(err)
	}
	ps, _ := fix.Pairs(raw)
	if fix.Get(ps, 35) != "0" || fix.Get(ps, 34) != "1" {
		t.Fatalf("incorrect idle heartbeat: %v", ps)
	}
	s.Close()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeat did not stop on disconnect")
	}
}
func await(t *testing.T, ch <-chan domain.Event, match func(domain.Event) bool) domain.Event {
	t.Helper()
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()
	for {
		select {
		case e := <-ch:
			if match(e) {
				return e
			}
		case <-timer.C:
			t.Fatal("event timeout")
			return domain.Event{}
		}
	}
}
func readFIX(c net.Conn) ([]byte, error) {
	_ = c.SetReadDeadline(time.Now().Add(4 * time.Second))
	b := []byte{}
	one := make([]byte, 1)
	for {
		_, e := c.Read(one)
		if e != nil {
			return nil, e
		}
		b = append(b, one[0])
		n, e := fix.FrameLength(b)
		if e != nil {
			return nil, e
		}
		if n > 0 {
			return b, nil
		}
	}
}

func TestTCPFramingAndScriptTimer(t *testing.T) {
	l, port := listener(t)
	received := make(chan string, 20)
	peer := make(chan net.Conn, 1)
	go func() {
		c, e := l.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		peer <- c
		r := bufio.NewReader(c)
		for {
			line, e := r.ReadString('\n')
			if e != nil {
				return
			}
			received <- line
			_, _ = c.Write([]byte(line))
		}
	}()
	events := make(chan domain.Event, 200)
	opts := domain.SessionOptions{Service: domain.Service{ID: "tcp", Host: "127.0.0.1", Port: port, Framing: "custom", FrameFunctionID: "frame"}, Functions: []domain.Function{{ID: "frame", Kind: "frame", Code: `const i=bytes.indexOf(10); return i<0?0:i+1;`}, {ID: "response", Kind: "response", Code: `return data.toUpperCase();`}}, Emit: func(e domain.Event) { events <- e }}
	s, e := (Factory{}).Connect(opts)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	c := <-peer
	r := domain.Request{Encoding: "text", Data: "hello\n", ResponseFunctionID: "response", PreScript: `return data.replace("hello","world");`, PostScript: `vars.set("last", data); console.log("received",data);`}
	if e = s.Send(r); e != nil {
		t.Fatal(e)
	}
	if line := <-received; line != "world\n" {
		t.Fatal(line)
	}
	await(t, events, func(e domain.Event) bool { return e.Direction == "receive" && e.Text == "WORLD\n" })
	_, _ = c.Write([]byte("分"))
	_, _ = c.Write([]byte("包\nsecond\npartial"))
	await(t, events, func(e domain.Event) bool { return e.Direction == "receive" && e.Raw == "分包\n" })
	await(t, events, func(e domain.Event) bool { return e.Direction == "receive" && e.Raw == "second\n" })
	_, _ = c.Write([]byte("-end\n"))
	await(t, events, func(e domain.Event) bool { return e.Direction == "receive" && e.Raw == "partial-end\n" })
	r.PostScript = ""
	r.PreScript = `vars.set("n",0); timers.setInterval("pulse",50,()=>{ vars.set("n",vars.get("n")+1); socket.send(new TextEncoder().encode("pulse"+vars.get("n")+"\n")); });`
	r.Data = "start\n"
	if e = s.Send(r); e != nil {
		t.Fatal(e)
	}
	await(t, events, func(e domain.Event) bool { return e.Direction == "send" && e.Raw == "pulse1\n" })
	s.StopTask("pulse")
	if len(s.State().Tasks) != 0 {
		t.Fatal("task not cleared")
	}
	s.Close()
	if s.State().Connected {
		t.Fatal("still connected")
	}
}

func TestScriptTimeoutAndErrorsVisible(t *testing.T) {
	l, port := listener(t)
	go func() {
		c, e := l.Accept()
		if e == nil {
			defer c.Close()
			_, _ = bufio.NewReader(c).ReadString('\n')
		}
	}()
	events := make(chan domain.Event, 30)
	s, e := (Factory{}).Connect(domain.SessionOptions{Service: domain.Service{Host: "127.0.0.1", Port: port}, Emit: func(e domain.Event) { events <- e }})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	start := time.Now()
	e = s.Send(domain.Request{Data: "unused", PreScript: `while(true){}`})
	if e == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("script not interrupted: %v", e)
	}
	await(t, events, func(e domain.Event) bool { return e.Kind == "error" && strings.Contains(e.Text, "1500ms") })
	if e = s.Send(domain.Request{Data: "ok\n"}); e != nil {
		t.Fatal("VM failed to recover", e)
	}
}

func TestFIXLogonAndTestRequest(t *testing.T) {
	l, port := listener(t)
	responses := make(chan []byte, 4)
	fail := make(chan error, 1)
	go func() {
		c, e := l.Accept()
		if e != nil {
			fail <- e
			return
		}
		defer c.Close()
		logon, e := readFIX(c)
		if e != nil {
			fail <- e
			return
		}
		ps, _ := fix.Pairs(logon)
		if fix.Get(ps, 35) != "A" || fix.Get(ps, 34) != "1" {
			fail <- fmt.Errorf("invalid logon")
			return
		}
		_, e = c.Write(fix.Encode("A", "Fix", "FixClient", 1, []fix.Pair{{Tag: 98, Value: "0"}, {Tag: 108, Value: "30"}, {Tag: 141, Value: "Y"}}, time.Now()))
		if e != nil {
			fail <- e
			return
		}
		_, e = c.Write(fix.Encode("1", "Fix", "FixClient", 2, []fix.Pair{{Tag: 112, Value: "probe-1"}}, time.Now()))
		if e != nil {
			fail <- e
			return
		}
		b, e := readFIX(c)
		if e != nil {
			fail <- e
			return
		}
		responses <- b
		_, e = readFIX(c)
		if e != nil {
			return
		}
	}()
	xml, e := os.ReadFile("../../../assets/fixspec/FIX44.xml")
	if e != nil {
		t.Fatal(e)
	}
	events := make(chan domain.Event, 100)
	s, e := (Factory{}).Connect(domain.SessionOptions{Service: domain.Service{ID: "fix", Protocol: "fix", Host: "127.0.0.1", Port: port, Sender: "FixClient", Target: "Fix", Heartbeat: 30, AutoLogon: true, ResetOnLogon: true}, Dictionary: domain.DictionaryFile{XML: string(xml)}, Emit: func(e domain.Event) { events <- e }})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	select {
	case e := <-fail:
		t.Fatal(e)
	case b := <-responses:
		ps, _ := fix.Pairs(b)
		if fix.Get(ps, 35) != "0" || fix.Get(ps, 34) != "2" || fix.Get(ps, 112) != "probe-1" {
			t.Fatal("incorrect heartbeat", strconv.Itoa(len(b)))
		}
	case <-time.After(4 * time.Second):
		t.Fatal("FIX response timeout")
	}
	state := s.State()
	if !state.LoggedOn || state.NextSeq != 3 || state.ExpectedSeq != 3 {
		t.Fatalf("wrong FIX state: %+v", state)
	}
}

func TestBinaryProcessing(t *testing.T) {
	l, port := listener(t)
	got := make(chan []byte, 1)
	go func() {
		c, e := l.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		b := make([]byte, 4)
		n, _ := c.Read(b)
		got <- b[:n]
	}()
	s, e := (Factory{}).Connect(domain.SessionOptions{Service: domain.Service{Host: "127.0.0.1", Port: port}, Emit: func(domain.Event) {}})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.Send(domain.Request{Encoding: "hex", Data: "00 01 ff 80"}); e != nil {
		t.Fatal(e)
	}
	if b := <-got; !bytes.Equal(b, []byte{0, 1, 255, 128}) {
		t.Fatal("binary changed")
	}
}
