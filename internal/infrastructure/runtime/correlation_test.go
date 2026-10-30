package runtime

import (
	"github.com/dop251/goja"
	"tcpconnect/internal/domain"
	"tcpconnect/internal/domain/fix"
	"testing"
	"time"
)

func TestFIXRequestCorrelationAndScriptIsolation(t *testing.T) {
	d, _ := fix.ResolveDictionary("FIX.4.4", nil)
	events := []domain.Event{}
	s := &Session{dictionary: d, vm: goja.New(), functions: map[string]domain.Function{}, vars: map[string]any{}, tasks: map[string]*task{}, pending: map[int][]byte{}, expected: 1, opts: domain.SessionOptions{Service: domain.Service{Sender: "CLIENT", Target: "SERVER"}, Emit: func(e domain.Event) { events = append(events, e) }}}
	if err := s.initVM(); err != nil {
		t.Fatal(err)
	}
	makeRequest := func(id string) domain.Request {
		return domain.Request{ID: id, MessageType: "D", PostScript: `vars.set("matched", "` + id + `"); console.log("matched ` + id + `");`}
	}
	encode := func(typ string, seq int, ps []fix.Pair) []byte {
		return fix.Encode("FIX.4.4", typ, "SERVER", "CLIENT", seq, ps, time.Now())
	}
	a, b := makeRequest("A"), makeRequest("B")
	s.trackRequest(encode("D", 1, []fix.Pair{{Tag: 11, Value: "order-A"}}), a)
	s.trackRequest(encode("D", 2, []fix.Pair{{Tag: 11, Value: "order-B"}}), b)
	a.PostScript = `throw new Error("mutated")`
	// Responses arrive in reverse order, independently of which tab was last sent.
	for i, tc := range []struct{ id, order string }{{"B", "order-B"}, {"A", "order-A"}} {
		raw := encode("8", i+1, []fix.Pair{{Tag: 11, Value: tc.order}, {Tag: 37, Value: "server-" + tc.id}})
		if err := s.receive(raw); err != nil {
			t.Fatal(err)
		}
		if s.vars["matched"] != tc.id {
			t.Fatal("wrong post script", s.vars)
		}
		found := false
		for _, e := range events {
			if e.Direction == "receive" && e.RequestID == tc.id {
				found = true
			}
		}
		if !found {
			t.Fatal("missing request association")
		}
		if s.active.ID != "" {
			t.Fatal("request context leaked")
		}
	}
	// Server OrderID is learned for later execution updates.
	if r := s.matchRequest(encode("8", 3, []fix.Pair{{Tag: 37, Value: "server-A"}})); r.ID != "A" {
		t.Fatal("lost learned order ID")
	}
	if r := s.matchRequest(encode("3", 4, []fix.Pair{{Tag: 45, Value: "2"}})); r.ID != "B" {
		t.Fatal("lost referenced sequence")
	}
	if r := s.matchRequest(encode("0", 5, nil)); r.ID != "" {
		t.Fatal("unsolicited heartbeat attributed to a request")
	}
	if r := s.matchRequest(encode("8", 6, []fix.Pair{{Tag: 11, Value: "other-order"}})); r.ID != "" {
		t.Fatal("unknown push attributed to a request")
	}
}
func TestTCPFIFOAndBoundedHistory(t *testing.T) {
	s := &Session{}
	for _, id := range []string{"A", "B"} {
		s.trackRequest([]byte(id), domain.Request{ID: id})
	}
	for _, want := range []string{"A", "B", ""} {
		if r := s.matchRequest([]byte("reply")); r.ID != want {
			t.Fatalf("got %s want %s", r.ID, want)
		}
	}
	for i := 0; i < 300; i++ {
		s.trackRequest([]byte("ping"), domain.Request{ID: "A"})
	}
	if len(s.outbound) != 256 {
		t.Fatal("unbounded correlation history")
	}
}
func TestEventSchemaSnapshotPreservesCustomFields(t *testing.T) {
	d, _ := fix.ResolveDictionary("FIX.4.2", nil)
	nodes := []fix.Node{{Kind: "field", Name: "BusinessField", Number: 9001, Type: "STRING"}}
	s := &Session{dictionary: d}
	r := domain.Request{ID: "A", MessageType: "0", CustomNodes: &nodes}
	raw := fix.Encode("FIX.4.2", "0", "CLIENT", "SERVER", 1, []fix.Pair{{Tag: 9001, Value: "x"}, {Tag: 9999, Value: "unknown"}}, time.Now())
	snapshot := s.eventDefinition(raw, r)
	if len(snapshot.Messages) != 1 || len(snapshot.Messages[0].Nodes) != 1 {
		t.Fatal("missing effective message structure")
	}
	found := false
	for _, f := range snapshot.Fields {
		if f.Number == 9001 && f.Name == "BusinessField" {
			found = true
		}
	}
	if !found {
		t.Fatal("custom tag metadata lost")
	}
	nodes[0].Name = "Changed"
	if snapshot.Messages[0].Nodes[0].Name != "BusinessField" {
		t.Fatal("snapshot mutated")
	}
}
