package runtime

import (
	"encoding/json"
	"strconv"
	"tcpconnect/internal/domain"
	"tcpconnect/internal/domain/fix"
)

type requestContext struct {
	request domain.Request
	keys    map[string]bool
	seq     string
	typ     string
	bytes   int
}

// IDs are scoped to this connection. ClOrdID and OrigClOrdID share a namespace.
func correlationKeys(ps []fix.Pair) []string {
	out := []string{}
	for _, tag := range []int{11, 41, 37, 112, 262, 320, 322, 335, 584, 710, 790} {
		for _, p := range ps {
			if p.Tag == tag && p.Value != "" && len(p.Value) <= 1024 {
				prefix := strconv.Itoa(tag)
				if tag == 11 || tag == 41 {
					prefix = "order"
				}
				out = append(out, prefix+":"+p.Value)
				if len(out) == 64 {
					return out
				}
			}
		}
	}
	return out
}
func (s *Session) trackRequest(b []byte, r domain.Request) {
	if r.ID == "" {
		return
	}
	// Freeze a snapshot: editing the request after sending must not change its scripts/schema.
	data, _ := json.Marshal(r)
	var snapshot domain.Request
	_ = json.Unmarshal(data, &snapshot)
	c := requestContext{request: snapshot, keys: map[string]bool{}, bytes: len(data)}
	if s.dictionary != nil {
		ps, _ := fix.Pairs(b)
		c.seq = fix.Get(ps, 34)
		c.typ = fix.Get(ps, 35)
		for _, k := range correlationKeys(ps) {
			c.keys[k] = true
		}
	}
	s.outbound = append(s.outbound, c)
	size := 0
	start := len(s.outbound)
	for start > 0 && len(s.outbound)-start < 256 {
		next := s.outbound[start-1].bytes
		if size+next > 8<<20 {
			break
		}
		size += next
		start--
	}
	if start > 0 {
		s.outbound = append([]requestContext(nil), s.outbound[start:]...)
	}
}
func (s *Session) matchRequest(b []byte) domain.Request {
	if len(s.outbound) == 0 {
		return domain.Request{}
	}
	if s.dictionary == nil {
		r := s.outbound[0].request
		s.outbound[0] = requestContext{}
		s.outbound = s.outbound[1:]
		return r
	}
	ps, err := fix.Pairs(b)
	if err != nil {
		return domain.Request{}
	}
	keys := correlationKeys(ps)
	matched := -1
	// Prefer the most specific incoming identifier before older aliases.
	for _, k := range keys {
		for i := len(s.outbound) - 1; i >= 0; i-- {
			if s.outbound[i].keys[k] {
				matched = i
				break
			}
		}
		if matched >= 0 {
			break
		}
	}
	if matched < 0 && fix.Get(ps, 45) != "" {
		for i := len(s.outbound) - 1; i >= 0; i-- {
			if s.outbound[i].seq == fix.Get(ps, 45) {
				matched = i
				break
			}
		}
	}
	typ := fix.Get(ps, 35)
	if matched < 0 && (typ == "A" || typ == "5") {
		for i, c := range s.outbound {
			if c.typ == typ {
				matched = i
				break
			}
		}
	}
	if matched < 0 {
		return domain.Request{}
	}
	c := &s.outbound[matched]
	// Learn server-assigned OrderID for subsequent execution updates.
	for _, k := range keys {
		if len(c.keys) < 64 {
			c.keys[k] = true
		}
	}
	r := c.request
	if typ == "A" || typ == "5" {
		s.outbound = append(s.outbound[:matched], s.outbound[matched+1:]...)
	}
	return r
}

func (s *Session) eventDefinition(b []byte, r domain.Request) *fix.Dictionary {
	if s.dictionary == nil {
		return nil
	}
	d, err := r.Definition(s.dictionary)
	if err != nil {
		d = s.dictionary
	}
	ps, err := fix.Pairs(b)
	if err != nil {
		return nil
	}
	tags := map[int]bool{}
	for _, p := range ps {
		tags[p.Tag] = true
	}
	out := &fix.Dictionary{Version: d.Version}
	for _, f := range d.Fields {
		if tags[f.Number] {
			out.Fields = append(out.Fields, f)
		}
	}
	var prune func([]fix.Node) []fix.Node
	prune = func(ns []fix.Node) []fix.Node {
		var result []fix.Node
		for _, n := range ns {
			n.Children = prune(n.Children)
			if tags[n.Number] || len(n.Children) > 0 {
				result = append(result, n)
			}
		}
		return result
	}
	if m, ok := d.Message(fix.Get(ps, 35)); ok {
		m.Nodes = prune(m.Nodes)
		out.Messages = []fix.Message{m}
	}
	out.Header = prune(d.Header)
	out.Trailer = prune(d.Trailer)
	return out
}
