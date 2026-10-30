package fix

import (
	"fmt"
	"strconv"
	"strings"
)

// WithMessageNodes changes one request's schema without mutating the imported dictionary.
func (d *Dictionary) WithMessageNodes(msgType string, nodes *[]Node) (*Dictionary, error) {
	if nodes == nil {
		return d, nil
	}
	if _, ok := d.Message(msgType); !ok {
		return nil, fmt.Errorf("消息类型未定义: %s", msgType)
	}
	count := 0
	var validate func([]Node, int) error
	fields := map[int]Field{}
	protected := map[int]bool{8: true, 9: true, 10: true, 35: true, 34: true, 49: true, 52: true, 56: true, 43: true, 122: true}
	for _, n := range append(flattenNodes(d.Header), flattenNodes(d.Trailer)...) {
		if n.Number > 0 {
			protected[n.Number] = true
		}
	}
	var validateScope func([]Node, map[int]bool, map[string]bool, int) error
	validateScope = func(ns []Node, tags map[int]bool, names map[string]bool, depth int) error {
		if depth > 32 {
			return fmt.Errorf("字段结构超过32层")
		}
		for _, n := range ns {
			count++
			if count > 10000 {
				return fmt.Errorf("请求结构超过10000节点")
			}
			if strings.TrimSpace(n.Name) == "" || strings.ContainsAny(n.Name, "\x01|=\r\n") || names[n.Name] {
				return fmt.Errorf("字段名称为空、重复或非法: %s", n.Name)
			}
			names[n.Name] = true
			if n.Kind == "component" {
				if err := validateScope(n.Children, tags, names, depth+1); err != nil {
					return err
				}
				continue
			}
			if n.Kind != "field" && n.Kind != "group" {
				return fmt.Errorf("未知节点类型: %s", n.Kind)
			}
			if n.Number <= 0 || protected[n.Number] || tags[n.Number] || n.Type == "" {
				return fmt.Errorf("字段Tag重复、保留或非法: %s (%d)", n.Name, n.Number)
			}
			tags[n.Number] = true
			if f, ok := fields[n.Number]; ok && (f.Name != n.Name || f.Type != n.Type) {
				return fmt.Errorf("同一Tag定义冲突: %d", n.Number)
			}
			fields[n.Number] = Field{Number: n.Number, Name: n.Name, Type: n.Type, Enums: n.Enums}
			if n.Kind == "group" {
				if len(n.Children) == 0 || firstTag(n.Children) == 0 {
					return fmt.Errorf("重复组需要首字段: %s", n.Name)
				}
				if n.Type != "NUMINGROUP" && n.Type != "INT" {
					return fmt.Errorf("重复组计数类型必须为NUMINGROUP或INT")
				}
				if err := validate(n.Children, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	validate = func(ns []Node, depth int) error { return validateScope(ns, map[int]bool{}, map[string]bool{}, depth) }
	if err := validate(*nodes, 0); err != nil {
		return nil, err
	}
	out := *d
	out.Messages = append([]Message{}, d.Messages...)
	for i := range out.Messages {
		if out.Messages[i].Type == msgType {
			out.Messages[i].Nodes = *nodes
		}
	}
	out.Fields = make([]Field, 0, len(d.Fields)+len(fields))
	for _, f := range d.Fields {
		if _, ok := fields[f.Number]; !ok {
			out.Fields = append(out.Fields, f)
		}
	}
	for _, f := range fields {
		out.Fields = append(out.Fields, f)
	}
	return &out, nil
}

func flattenNodes(ns []Node) []Node {
	out := []Node{}
	for _, n := range ns {
		if n.Kind == "component" {
			out = append(out, flattenNodes(n.Children)...)
		} else {
			out = append(out, n)
		}
	}
	return out
}

// TextBody validates against the effective schema and returns all original body pairs in order.
func (d *Dictionary) TextBody(msgType, raw string) ([]Pair, error) {
	raw = strings.ReplaceAll(raw, "\r", "")
	raw = strings.ReplaceAll(raw, "\n", "")
	if !strings.Contains(raw, SOH) {
		raw = strings.ReplaceAll(raw, "|", SOH)
	}
	ps, err := Pairs([]byte(raw))
	if err != nil {
		return nil, err
	}
	if Get(ps, 35) != msgType {
		return nil, fmt.Errorf("报文35与所选消息类型不一致")
	}
	if version := Get(ps, 8); version != "" && version != d.Version {
		return nil, fmt.Errorf("报文协议与服务协议不一致")
	}
	m, ok := d.Message(msgType)
	if !ok {
		return nil, fmt.Errorf("消息类型未定义")
	}
	body := BodyPairs(ps)
	header := map[int]bool{43: true, 122: true}
	for _, n := range append(flattenNodes(d.Header), flattenNodes(d.Trailer)...) {
		header[n.Number] = true
	}
	filtered := []Pair{}
	for _, p := range body {
		if !header[p.Tag] {
			filtered = append(filtered, p)
		}
	}
	var parse func([]Node, *int, int) (map[string]any, error)
	parse = func(nodes []Node, index *int, delimiter int) (map[string]any, error) {
		ns := flattenNodes(nodes)
		byTag := map[int]Node{}
		for _, n := range ns {
			byTag[n.Number] = n
		}
		values := map[string]any{}
		for *index < len(filtered) {
			p := filtered[*index]
			n, found := byTag[p.Tag]
			if !found {
				if delimiter != 0 {
					break
				}
				return nil, fmt.Errorf("Tag %d 未在当前消息定义中，请先在编辑视图添加字段", p.Tag)
			}
			if _, exists := values[n.Name]; exists {
				if p.Tag == delimiter {
					break
				}
				return nil, fmt.Errorf("非重复组字段重复: %s", n.Name)
			}
			(*index)++
			if n.Kind != "group" {
				values[n.Name] = p.Value
				continue
			}
			count, e := strconv.Atoi(p.Value)
			if e != nil || count < 0 || count > 1000 {
				return nil, fmt.Errorf("重复组计数无效: %s", n.Name)
			}
			rows := []any{}
			first := firstTag(n.Children)
			for i := 0; i < count; i++ {
				if *index >= len(filtered) || filtered[*index].Tag != first {
					return nil, fmt.Errorf("重复组%s记录不足或缺少首字段", n.Name)
				}
				row, e := parse(n.Children, index, first)
				if e != nil {
					return nil, e
				}
				rows = append(rows, row)
			}
			values[n.Name] = rows
		}
		return values, nil
	}
	index := 0
	values, err := parse(m.Nodes, &index, 0)
	if err != nil {
		return nil, err
	}
	if _, err = d.Body(msgType, values); err != nil {
		return nil, err
	}
	return body, nil
}
