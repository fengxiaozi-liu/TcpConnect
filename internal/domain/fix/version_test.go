package fix

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestClassicDictionariesAndWireVersions(t *testing.T) {
	for minor := 0; minor <= 4; minor++ {
		t.Run(fmt.Sprintf("FIX4%d", minor), func(t *testing.T) {
			xml, err := os.ReadFile(fmt.Sprintf("../../../assets/fixspec/FIX4%d.xml", minor))
			if err != nil {
				t.Fatal(err)
			}
			d, err := ParseDictionary(xml)
			if err != nil {
				t.Fatal(err)
			}
			version := fmt.Sprintf("FIX.4.%d", minor)
			if d.Version != version {
				t.Fatalf("wrong dictionary version: %s", d.Version)
			}
			body, err := d.Body("D", map[string]any{
				"ClOrdID": "ORDER-123", "HandlInst": "1", "Account": "ACCOUNT",
				"Symbol": "SYMBOL", "Side": "1", "OrdType": "1", "OrderQty": "10",
				"TransactTime": "20261029-07:00:00",
			})
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 29, 15, 0, 0, 123000000, time.FixedZone("CST", 8*3600))
			raw := Encode(d.Version, "D", "CLIENT", "SERVER", 1, body, now)
			ps, _ := Pairs(raw)
			if Get(ps, 8) != version || Get(ps, 11) != "ORDER-123" || Get(ps, 55) != "SYMBOL" {
				t.Fatalf("wrong serialized order: %s", raw)
			}
			ts := "20261029-07:00:00"
			if minor >= 2 {
				ts += ".123"
			}
			if Get(ps, 52) != ts {
				t.Fatalf("wrong SendingTime: %s", Get(ps, 52))
			}
			for i := 0; i < len(raw); i++ {
				if n, err := FrameLength(raw[:i]); n != 0 || err != nil {
					t.Fatalf("partial frame %d: %d %v", i, n, err)
				}
			}
			joined := append(bytes.Clone(raw), raw...)
			if n, err := FrameLength(joined); n != len(raw) || err != nil {
				t.Fatalf("joined frames: %d %v", n, err)
			}
			corrupt := bytes.Clone(raw)
			corrupt[bytes.Index(corrupt, []byte("ORDER-123"))] = 'X'
			if _, err := FrameLength(corrupt); err == nil {
				t.Fatal("corrupt checksum accepted")
			}
		})
	}
}

func TestDictionaryVersionValidation(t *testing.T) {
	for _, attrs := range []string{`type="FIX" major="5" minor="0"`, `type="FIXT" major="1" minor="1"`, `type="FIX" major="4" minor="9"`} {
		_, err := ParseDictionary([]byte(`<fix ` + attrs + `><messages><message name="Heartbeat" msgtype="0"/></messages></fix>`))
		if err == nil || !strings.Contains(err.Error(), "FIX.4.0～FIX.4.4") {
			t.Fatalf("unsupported version accepted: %s %v", attrs, err)
		}
	}
	if _, err := FrameLength([]byte("8=FIXT.1.1\x019=0\x01")); err == nil {
		t.Fatal("unsupported transport accepted")
	}
	d, err := ParseDictionary([]byte(`<fix major="4" minor="2"><messages><message name="Heartbeat" msgtype="0"/></messages></fix>`))
	if err != nil || d.Version != "FIX.4.2" {
		t.Fatalf("legacy default type: %v", err)
	}
}

func TestLegacyFieldValidation(t *testing.T) {
	for _, tc := range []struct{ typ, good, bad string }{
		{"TIME", "20261029-07:00:00", "07:00:00"},
		{"DATE", "20261029", "20260230"},
		{"UTCDATE", "20261029", "2026-10-29"},
		{"QUANTITY", "123.5", "no"},
	} {
		n := Node{Name: "test", Type: tc.typ}
		if Validate(n, tc.good) != nil || Validate(n, tc.bad) == nil {
			t.Fatalf("invalid validation for %s", tc.typ)
		}
	}
	if Validate(Node{Type: "CHAR"}, "MULTI") == nil {
		t.Fatal("modern CHAR length restriction lost")
	}
}

func TestBuiltinDefinitionsAndXMLVersion(t *testing.T) {
	for minor := 0; minor <= 4; minor++ {
		version := fmt.Sprintf("FIX.4.%d", minor)
		d, err := ResolveDictionary(version, nil)
		if err != nil {
			t.Fatal(err)
		}
		if d.Version != version {
			t.Fatal("builtin version mismatch")
		}
		ps, err := d.Body("D", map[string]any{"ClOrdID": "BUILTIN-ORDER", "HandlInst": "1", "Symbol": "TEST", "Side": "1", "OrderQty": "10", "OrdType": "1", "TransactTime": "20261030-01:02:03"})
		if err != nil || Get(ps, 11) != "BUILTIN-ORDER" {
			t.Fatalf("builtin order %s: %v", version, err)
		}
	}
	xml := []byte(`<fix major='4' type='FIX' servicepack='0' minor='2'><messages><message name='Heartbeat' msgtype='0'/></messages></fix>`)
	d, err := ResolveDictionary("", xml)
	if err != nil || d.Version != "FIX.4.2" {
		t.Fatalf("XML root identification: %v", err)
	}
	if _, err := ResolveDictionary("FIX.4.4", xml); err == nil {
		t.Fatal("mismatched explicit version accepted")
	}
	if _, err := ResolveDictionary("FIX.5.0", nil); err == nil {
		t.Fatal("unsupported builtin version accepted")
	}
}
