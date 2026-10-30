package fix

import (
	"reflect"
	"strings"
	"testing"
)

func TestRequestSchemaAndManagedText(t *testing.T) {
	d, err := ResolveDictionary("FIX.4.2", nil)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []Node{{Kind: "field", Name: "Probe", Number: 9001, Type: "STRING", Required: true}, {Kind: "group", Name: "Rows", Number: 9002, Type: "NUMINGROUP", Children: []Node{{Kind: "field", Name: "RowID", Number: 9003, Type: "STRING", Required: true}, {Kind: "field", Name: "Size", Number: 9004, Type: "INT"}}}}
	custom, err := d.WithMessageNodes("0", &nodes)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := d.Message("0")
	if reflect.DeepEqual(original.Nodes, nodes) {
		t.Fatal("mutated original dictionary")
	}
	raw := "8=FIX.4.2|35=0|9001=hello=world|9002=2|9003=a|9004=10|9003=b|9004=20|10=000|"
	pairs, err := custom.TextBody("0", raw)
	if err != nil {
		t.Fatal(err)
	}
	want := []Pair{{9001, "hello=world"}, {9002, "2"}, {9003, "a"}, {9004, "10"}, {9003, "b"}, {9004, "20"}}
	if !reflect.DeepEqual(pairs, want) {
		t.Fatalf("lost fields: %v", pairs)
	}
	for _, test := range []struct{ raw, contains string }{{"35=0|9001=a|9999=x|", "未在当前消息定义"}, {"35=0|9002=0|", "必填字段"}, {"35=0|9001=a|9002=2|9003=a|", "记录不足"}, {"35=0|9001=a|9002=1|9003=a|9004=bad|", "不符合"}, {"35=A|9001=a|", "不一致"}, {"8=FIX.4.4|35=0|9001=a|", "不一致"}, {"35=0|9001=a|9001=b|", "重复"}} {
		if _, err := custom.TextBody("0", test.raw); err == nil || !strings.Contains(err.Error(), test.contains) {
			t.Fatalf("%s: %v", test.raw, err)
		}
	}
	empty := []Node{}
	removed, err := d.WithMessageNodes("A", &empty)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = removed.Body("A", nil); err != nil {
		t.Fatal(err)
	}
}
func TestInvalidRequestSchema(t *testing.T) {
	d, _ := ResolveDictionary("FIX.4.4", nil)
	for _, ns := range [][]Node{
		{{Kind: "field", Name: "BadHeader", Number: 52, Type: "STRING"}},
		{{Kind: "field", Name: "A", Number: 9001, Type: "STRING"}, {Kind: "field", Name: "B", Number: 9001, Type: "STRING"}},
		{{Kind: "component", Name: "C", Children: []Node{{Kind: "field", Name: "A", Number: 9001, Type: "STRING"}}}, {Kind: "field", Name: "A", Number: 9002, Type: "STRING"}},
		{{Kind: "group", Name: "Rows", Number: 9002, Type: "NUMINGROUP"}},
	} {
		if _, err := d.WithMessageNodes("0", &ns); err == nil {
			t.Fatal("accepted invalid schema")
		}
	}
}
