package runtime

import (
	"bufio"
	"net"
	"tcpconnect/internal/domain"
	"tcpconnect/internal/domain/fix"
	"testing"
	"time"
)

func TestRepeatRequestUsesSameConnectionAndSnapshot(t *testing.T) {
	l, port := listener(t)
	lines := make(chan string, 10)
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		reader := bufio.NewReader(c)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			lines <- line
			c.Write([]byte(line))
		}
	}()
	events := make(chan domain.Event, 100)
	connected, err := (Factory{}).Connect(domain.SessionOptions{Service: domain.Service{ID: "repeat", Host: "127.0.0.1", Port: port, Framing: "delimiter", Delimiter: "\n"}, Emit: func(e domain.Event) { events <- e }})
	if err != nil {
		t.Fatal(err)
	}
	defer connected.Close()
	r := domain.Request{ID: "pulse", ServiceID: "repeat", Name: "Heartbeat", Data: "ping\n", Encoding: "text", PreScript: `return data.replace("ping","heartbeat");`, PostScript: `console.log("pulse received");`}
	if err = connected.StartRepeat(r, 0); err == nil {
		t.Fatal("accepted zero interval")
	}
	if err = connected.StartRepeat(r, 1); err != nil {
		t.Fatal(err)
	}
	r.Data = "changed\n"
	state := connected.State()
	if len(state.TaskDetails) != 1 || state.TaskDetails[0].IntervalMS != 1000 || state.TaskDetails[0].Kind != "request" || state.TaskDetails[0].RequestID != "pulse" {
		t.Fatalf("missing task metadata: %+v", state)
	}
	received := await(t, events, func(e domain.Event) bool { return e.Direction == "receive" })
	if received.RequestID != "pulse" || received.Raw != "heartbeat\n" {
		t.Fatalf("wrong payload/context: %+v", received)
	}
	await(t, events, func(e domain.Event) bool { return e.Kind == "log" && e.RequestID == "pulse" })
	connected.StopTask("request/pulse")
	if len(connected.State().Tasks) != 0 {
		t.Fatal("stop failed")
	}
	<-lines
	select {
	case line := <-lines:
		t.Fatal("sent after stop", line)
	case <-time.After(1100 * time.Millisecond):
	}
	r.PreScript = `throw new Error("periodic failure")`
	if err = connected.StartRepeat(r, 1); err != nil {
		t.Fatal(err)
	}
	failure := await(t, events, func(e domain.Event) bool { return e.Kind == "error" })
	if failure.RequestID != "pulse" {
		t.Fatal("periodic failure lost request context")
	}
	if len(connected.State().Tasks) != 0 {
		t.Fatal("failed task not stopped")
	}
	r.PreScript = ""
	if err = connected.StartRepeat(r, 1); err != nil {
		t.Fatal(err)
	}
	connected.Close()
	if len(connected.State().Tasks) != 0 {
		t.Fatal("disconnect leaked tasks")
	}
	if err = connected.StartRepeat(r, 1); err == nil {
		t.Fatal("started while disconnected")
	}
}

func TestFIXHeartbeatVisibleWithoutScriptTask(t *testing.T) {
	client, peer := net.Pipe()
	defer peer.Close()
	d, _ := fix.ResolveDictionary("FIX.4.2", nil)
	s := &Session{dictionary: d, conn: client, connected: true, seq: 1, tasks: map[string]*task{}, journal: map[int][]byte{}, lastRead: time.Now(), lastWrite: time.Now(), opts: domain.SessionOptions{Service: domain.Service{ID: "fix", Sender: "CLIENT", Target: "SERVER", Heartbeat: 1}, Emit: func(domain.Event) {}}}
	defer s.Close()
	if state := s.State(); state.Heartbeat == nil || state.Heartbeat.Status != "waiting_logon" {
		t.Fatal("missing waiting state")
	}
	request := domain.Request{ID: "heartbeat", ServiceID: "fix", MessageType: "0"}
	if err := s.StartRepeat(request, 1); err == nil {
		t.Fatal("started managed FIX before login")
	}
	s.loggedOn = true
	if s.State().Heartbeat.Status != "running" {
		t.Fatal("missing running state")
	}
	done := make(chan error, 1)
	go func() { s.mu.Lock(); defer s.mu.Unlock(); done <- s.sendAdmin("0", nil) }()
	raw, err := readFIX(peer)
	if err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	ps, _ := fix.Pairs(raw)
	if fix.Get(ps, 35) != "0" || fix.Get(ps, 8) != "FIX.4.2" {
		t.Fatal("incorrect heartbeat")
	}
	if s.State().Heartbeat.LastSentAt == nil {
		t.Fatal("missing last heartbeat time")
	}
	s.Close()
	if s.State().Heartbeat.Status != "disconnected" {
		t.Fatal("missing disconnected state")
	}
}
