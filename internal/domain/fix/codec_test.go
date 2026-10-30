package fix

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestImportedDictionary(t *testing.T) {
	b, e := os.ReadFile("../../../assets/fixspec/FIX44.xml")
	if e != nil {
		t.Fatal(e)
	}
	d, e := ParseDictionary(b)
	if e != nil {
		t.Fatal(e)
	}
	if len(d.Messages) != 181 || len(d.Fields) != 1262 || d.ComponentCount != 26 {
		t.Fatalf("unexpected dictionary: %d/%d/%d", len(d.Messages), len(d.Fields), d.ComponentCount)
	}
	m, ok := d.Message("D")
	if !ok {
		t.Fatal("NewOrderSingle absent")
	}
	account := false
	custom := false
	for _, n := range m.Nodes {
		if n.Name == "Account" {
			account = n.Required
		}
		if n.Number == 8006 {
			custom = true
		}
	}
	if !account || !custom {
		t.Fatal("custom required metadata lost")
	}
	_, e = d.Body("A", map[string]any{"EncryptMethod": "0", "HeartBtInt": "30"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = d.Body("A", map[string]any{"EncryptMethod": "0"})
	if e == nil {
		t.Fatal("required field accepted")
	}
	ps, e := d.Body("A", map[string]any{"EncryptMethod": "0", "HeartBtInt": "30", "NoMsgTypes": []any{map[string]any{"RefMsgType": "D", "MsgDirection": "S"}}})
	if e != nil {
		t.Fatal(e)
	}
	if Get(ps, 384) != "1" {
		t.Fatal("group count not generated")
	}
}

func TestByteFraming(t *testing.T) {
	raw := Encode("FIX.4.4", "0", "CLIENT", "SERVER", 2, []Pair{{112, "测试"}}, time.Now())
	n, e := FrameLength(raw)
	if e != nil || n != len(raw) {
		t.Fatalf("%d %v", n, e)
	}
	for i := 0; i < len(raw); i++ {
		n, e = FrameLength(raw[:i])
		if e != nil || n != 0 {
			t.Fatalf("split %d: %d %v", i, n, e)
		}
	}
	joined := append(append([]byte{}, raw...), raw...)
	n, e = FrameLength(joined)
	if e != nil || n != len(raw) {
		t.Fatal("sticky packets not separated", e)
	}
	bad := bytes.Clone(raw)
	bad[len(bad)-3] = '9'
	if _, e = FrameLength(bad); e == nil {
		t.Fatal("checksum corruption accepted")
	}
}

func TestDictionaryRejectsCycle(t *testing.T) {
	_, e := ParseDictionary([]byte(`<fix type="FIX" major="4" minor="4"><messages><message name="Test" msgtype="U"><component name="A"/></message></messages><components><component name="A"><component name="A"/></component></components></fix>`))
	if e == nil {
		t.Fatal("cycle accepted")
	}
}

func TestComponentAndGroupPresence(t *testing.T) {
	d := &Dictionary{Messages: []Message{{Type: "U", Nodes: []Node{
		{Kind: "component", Name: "Optional", Children: []Node{{Kind: "field", Name: "Mandatory", Number: 5000, Required: true, Type: "STRING"}}},
		{Kind: "group", Name: "Rows", Number: 5001, Children: []Node{{Kind: "field", Name: "First", Number: 5002, Type: "STRING"}, {Kind: "field", Name: "Other", Number: 5003, Type: "STRING"}}},
	}}}}
	if _, err := d.Body("U", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Body("U", map[string]any{"__component_Optional": true}); err == nil {
		t.Fatal("enabled component omitted required child")
	}
	if _, err := d.Body("U", map[string]any{"Rows": []any{map[string]any{"Other": "value"}}}); err == nil {
		t.Fatal("group without delimiter accepted")
	}
	ps, err := d.Body("U", map[string]any{"Rows": []any{map[string]any{"First": "one"}, map[string]any{"First": "two"}}})
	if err != nil || Get(ps, 5001) != "2" || len(ps) != 3 {
		t.Fatalf("group serialization: %v %v", ps, err)
	}
}

func TestMultipleValueEnums(t *testing.T) {
	n := Node{Name: "ExecInst", Type: "MULTIPLEVALUESTRING", Enums: []Enum{{Value: "1"}, {Value: "2"}}}
	if err := Validate(n, "1 2"); err != nil {
		t.Fatal(err)
	}
	if err := Validate(n, "1 3"); err == nil {
		t.Fatal("unknown enum accepted")
	}
}
