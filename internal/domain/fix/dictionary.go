package fix

import (
	"encoding/xml"
	"fmt"
	"strconv"
)

type Enum struct {
	Value       string `xml:"enum,attr" json:"value"`
	Description string `xml:"description,attr" json:"description"`
}
type Field struct {
	Number int    `xml:"number,attr" json:"number"`
	Name   string `xml:"name,attr" json:"name"`
	Type   string `xml:"type,attr" json:"type"`
	Enums  []Enum `xml:"value" json:"enums"`
}
type Node struct {
	Description string `json:"description,omitempty"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Number      int    `json:"number,omitempty"`
	Type        string `json:"type,omitempty"`
	Enums       []Enum `json:"enums,omitempty"`
	Children    []Node `json:"children,omitempty"`
}
type Message struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Category string `json:"category"`
	Nodes    []Node `json:"nodes"`
}
type Dictionary struct {
	Version        string    `json:"version"`
	Messages       []Message `json:"messages"`
	Fields         []Field   `json:"fields"`
	Header         []Node    `json:"header"`
	Trailer        []Node    `json:"trailer"`
	ComponentCount int       `json:"componentCount"`
}
type ref struct {
	XMLName  xml.Name
	Name     string `xml:"name,attr"`
	Required string `xml:"required,attr"`
	Children []ref  `xml:",any"`
}
type messageXML struct {
	Name     string `xml:"name,attr"`
	Type     string `xml:"msgtype,attr"`
	Category string `xml:"msgcat,attr"`
	Children []ref  `xml:",any"`
}

func ParseDictionary(data []byte) (*Dictionary, error) {
	if len(data) > 8<<20 {
		return nil, fmt.Errorf("字典超过 8 MiB")
	}
	var root struct {
		XMLName xml.Name `xml:"fix"`
		Major   string   `xml:"major,attr"`
		Minor   string   `xml:"minor,attr"`
		Type    string   `xml:"type,attr"`
		Header  struct {
			Children []ref `xml:",any"`
		} `xml:"header"`
		Trailer struct {
			Children []ref `xml:",any"`
		} `xml:"trailer"`
		Messages   []messageXML `xml:"messages>message"`
		Components []ref        `xml:"components>component"`
		Fields     []Field      `xml:"fields>field"`
	}
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("XML 解析失败: %w", err)
	}
	// Older QuickFIX dictionaries may omit type, whose default is FIX.
	if root.Type == "" {
		root.Type = "FIX"
	}
	version := root.Type + "." + root.Major + "." + root.Minor
	if !SupportsVersion(version) {
		return nil, fmt.Errorf("支持 QuickFIX FIX.4.0～FIX.4.4 字典，收到 %s；FIX 5.x/FIXT 会话暂不支持", version)
	}
	d := &Dictionary{Version: version, Fields: root.Fields, ComponentCount: len(root.Components)}
	fields := map[string]Field{}
	numbers := map[int]bool{}
	components := map[string]ref{}
	for _, f := range root.Fields {
		if f.Name == "" || f.Number <= 0 || numbers[f.Number] || fields[f.Name].Number != 0 {
			return nil, fmt.Errorf("字段重复或非法: %s (%d)", f.Name, f.Number)
		}
		fields[f.Name] = f
		numbers[f.Number] = true
	}
	for _, c := range root.Components {
		if _, ok := components[c.Name]; ok {
			return nil, fmt.Errorf("组件重复: %s", c.Name)
		}
		components[c.Name] = c
	}
	var resolve func([]ref, map[string]bool, int) ([]Node, error)
	totalNodes := 0
	resolve = func(refs []ref, stack map[string]bool, depth int) ([]Node, error) {
		if depth > 32 {
			return nil, fmt.Errorf("字典嵌套超过 32 层")
		}
		out := make([]Node, 0, len(refs))
		for _, r := range refs {
			totalNodes++
			if totalNodes > 200000 {
				return nil, fmt.Errorf("字典展开超过 200000 节点")
			}
			n := Node{Kind: r.XMLName.Local, Name: r.Name, Required: r.Required == "Y"}
			switch n.Kind {
			case "field", "group":
				f, ok := fields[r.Name]
				if !ok {
					return nil, fmt.Errorf("字段未定义: %s", r.Name)
				}
				n.Number = f.Number
				n.Type = f.Type
				n.Enums = f.Enums
				if n.Kind == "group" {
					var err error
					n.Children, err = resolve(r.Children, stack, depth+1)
					if err != nil {
						return nil, err
					}
				}
			case "component":
				c, ok := components[r.Name]
				if !ok {
					return nil, fmt.Errorf("组件未定义: %s", r.Name)
				}
				if stack[r.Name] {
					return nil, fmt.Errorf("组件循环引用: %s", r.Name)
				}
				stack[r.Name] = true
				var err error
				n.Children, err = resolve(c.Children, stack, depth+1)
				delete(stack, r.Name)
				if err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("不支持的字典节点: %s", n.Kind)
			}
			out = append(out, n)
		}
		return out, nil
	}
	var err error
	if d.Header, err = resolve(root.Header.Children, map[string]bool{}, 0); err != nil {
		return nil, err
	}
	if d.Trailer, err = resolve(root.Trailer.Children, map[string]bool{}, 0); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, m := range root.Messages {
		if m.Name == "" || m.Type == "" || seen[m.Type] {
			return nil, fmt.Errorf("消息重复或非法: %s", m.Name)
		}
		seen[m.Type] = true
		nodes, e := resolve(m.Children, map[string]bool{}, 0)
		if e != nil {
			return nil, fmt.Errorf("%s: %w", m.Name, e)
		}
		d.Messages = append(d.Messages, Message{m.Name, m.Type, m.Category, nodes})
	}
	if len(d.Messages) == 0 {
		return nil, fmt.Errorf("字典没有消息定义")
	}
	return d, nil
}

func (d *Dictionary) Message(msgType string) (Message, bool) {
	for _, m := range d.Messages {
		if m.Type == msgType {
			return m, true
		}
	}
	return Message{}, false
}

type Pair struct {
	Tag   int    `json:"tag"`
	Value string `json:"value"`
}

func (d *Dictionary) Body(msgType string, values map[string]any) ([]Pair, error) {
	m, ok := d.Message(msgType)
	if !ok {
		return nil, fmt.Errorf("消息类型未定义: %s", msgType)
	}
	var present func([]Node, map[string]any) bool
	present = func(ns []Node, v map[string]any) bool {
		for _, n := range ns {
			if n.Kind == "component" {
				if v["__component_"+n.Name] == true || present(n.Children, v) {
					return true
				}
			} else if x, ok := v[n.Name]; ok && x != nil && fmt.Sprint(x) != "" {
				return true
			}
		}
		return false
	}
	var walk func([]Node, map[string]any) ([]Pair, error)
	walk = func(ns []Node, v map[string]any) ([]Pair, error) {
		out := []Pair{}
		for _, n := range ns {
			if n.Kind == "component" {
				if !n.Required && v["__component_"+n.Name] != true && !present(n.Children, v) {
					continue
				}
				ps, e := walk(n.Children, v)
				if e != nil {
					return nil, e
				}
				out = append(out, ps...)
				continue
			}
			x, exists := v[n.Name]
			if !exists || x == nil || fmt.Sprint(x) == "" {
				if n.Required {
					return nil, fmt.Errorf("必填字段: %s (%d)", n.Name, n.Number)
				}
				continue
			}
			if n.Kind == "group" {
				rows, ok := x.([]any)
				if !ok {
					return nil, fmt.Errorf("重复组 %s 必须为数组", n.Name)
				}
				if len(rows) == 0 {
					if n.Required {
						return nil, fmt.Errorf("必填重复组: %s", n.Name)
					}
					continue
				}
				if len(rows) > 1000 {
					return nil, fmt.Errorf("重复组超过 1000 条")
				}
				out = append(out, Pair{n.Number, strconv.Itoa(len(rows))})
				for _, row := range rows {
					v, ok := row.(map[string]any)
					if !ok {
						return nil, fmt.Errorf("重复组记录必须为对象")
					}
					ps, e := walk(n.Children, v)
					if e != nil {
						return nil, e
					}
					if len(ps) == 0 || ps[0].Tag != firstTag(n.Children) {
						return nil, fmt.Errorf("重复组 %s 的每条记录必须填写首字段 (%d)", n.Name, firstTag(n.Children))
					}
					out = append(out, ps...)
				}
				continue
			}
			value := fmt.Sprint(x)
			validationNode := n
			// In pre-4.2 dictionaries CHAR also describes strings such as Symbol and ClOrdID.
			if (d.Version == "FIX.4.0" || d.Version == "FIX.4.1") && n.Type == "CHAR" {
				validationNode.Type = "STRING"
			}
			if err := Validate(validationNode, value); err != nil {
				return nil, err
			}
			out = append(out, Pair{n.Number, value})
		}
		return out, nil
	}
	return walk(m.Nodes, values)
}

func firstTag(nodes []Node) int {
	for _, n := range nodes {
		if n.Kind != "component" {
			return n.Number
		}
		if tag := firstTag(n.Children); tag != 0 {
			return tag
		}
	}
	return 0
}
