package fix

import (
	"bytes"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

const SOH = "\x01"
const MaxFrame = 4 << 20

func Validate(n Node, v string) error {
	bad := func() error { return fmt.Errorf("字段 %s (%d) 的值不符合 %s", n.Name, n.Number, n.Type) }
	if strings.ContainsAny(v, "\x01\r\n") {
		return bad()
	}
	if len(n.Enums) > 0 {
		values := []string{v}
		if strings.HasPrefix(n.Type, "MULTIPLE") {
			values = strings.Fields(v)
		}
		if len(values) == 0 {
			return bad()
		}
		for _, value := range values {
			ok := false
			for _, e := range n.Enums {
				if e.Value == value {
					ok = true
					break
				}
			}
			if !ok {
				return bad()
			}
		}
	}
	switch n.Type {
	case "INT", "LENGTH", "NUMINGROUP", "SEQNUM", "DAYOFMONTH":
		i, e := strconv.ParseInt(v, 10, 64)
		if e != nil {
			return bad()
		}
		if n.Type != "INT" && i < 0 {
			return bad()
		}
	case "FLOAT", "QTY", "PRICE", "PRICEOFFSET", "AMT", "PERCENTAGE":
		if _, ok := new(big.Rat).SetString(v); !ok || strings.Contains(v, "/") {
			return bad()
		}
	case "BOOLEAN":
		if v != "Y" && v != "N" {
			return bad()
		}
	case "CHAR":
		if len([]rune(v)) != 1 {
			return bad()
		}
	case "UTCTIMESTAMP":
		if _, e := time.Parse("20060102-15:04:05", v); e != nil {
			return bad()
		}
	case "UTCDATEONLY", "LOCALMKTDATE":
		if _, e := time.Parse("20060102", v); e != nil {
			return bad()
		}
	case "UTCTIMEONLY":
		if _, e := time.Parse("15:04:05", v); e != nil {
			return bad()
		}
	}
	return nil
}

func Encode(msgType, sender, target string, seq int, body []Pair, now time.Time) []byte {
	ps := []Pair{{35, msgType}, {49, sender}, {56, target}, {34, strconv.Itoa(seq)}}
	for _, p := range body {
		if p.Tag == 43 {
			ps = append(ps, p)
		}
	}
	ps = append(ps, Pair{52, now.UTC().Format("20060102-15:04:05.000")})
	for _, p := range body {
		if p.Tag == 122 {
			ps = append(ps, p)
		}
	}
	for _, p := range body {
		if p.Tag != 43 && p.Tag != 122 {
			ps = append(ps, p)
		}
	}
	var b strings.Builder
	for _, p := range ps {
		fmt.Fprintf(&b, "%d=%s%s", p.Tag, p.Value, SOH)
	}
	raw := fmt.Sprintf("8=FIX.4.4%s9=%d%s%s", SOH, b.Len(), SOH, b.String())
	sum := 0
	for _, c := range []byte(raw) {
		sum = (sum + int(c)) % 256
	}
	return []byte(fmt.Sprintf("%s10=%03d%s", raw, sum, SOH))
}

// FrameLength returns the complete frame size; zero means more bytes are needed.
func FrameLength(data []byte) (int, error) {
	if len(data) > MaxFrame {
		return 0, fmt.Errorf("FIX 接收缓冲超过 4 MiB")
	}
	if len(data) < len("8=FIX.4.4\x01") {
		if !bytes.HasPrefix([]byte("8=FIX.4.4\x01"), data) {
			return 0, fmt.Errorf("FIX 报文头无效")
		}
		return 0, nil
	}
	if !bytes.HasPrefix(data, []byte("8=FIX.4.4\x019=")) {
		if len(data) < 12 {
			return 0, nil
		}
		return 0, fmt.Errorf("FIX 必须以 8=FIX.4.4、9= 开头")
	}
	start := len("8=FIX.4.4\x019=")
	end := bytes.IndexByte(data[start:], 1)
	if end < 0 {
		if len(data) > 40 {
			return 0, fmt.Errorf("BodyLength 无效")
		}
		return 0, nil
	}
	end += start
	n, e := strconv.Atoi(string(data[start:end]))
	if e != nil || n < 0 || n > MaxFrame {
		return 0, fmt.Errorf("BodyLength 无效")
	}
	total := end + 1 + n + 7
	if total > MaxFrame {
		return 0, fmt.Errorf("FIX 报文过大")
	}
	if len(data) < total {
		return 0, nil
	}
	tail := data[end+1+n : total]
	if string(tail[:3]) != "10=" || tail[6] != 1 {
		return 0, fmt.Errorf("BodyLength 与 CheckSum 位置不符")
	}
	sum, e := strconv.Atoi(string(tail[3:6]))
	if e != nil || sum < 0 || sum > 255 {
		return 0, fmt.Errorf("CheckSum 格式错误")
	}
	actual := 0
	for _, b := range data[:total-7] {
		actual = (actual + int(b)) % 256
	}
	if actual != sum {
		return 0, fmt.Errorf("CheckSum 错误: 期望 %03d，收到 %03d", actual, sum)
	}
	return total, nil
}

func Pairs(raw []byte) ([]Pair, error) {
	ps := []Pair{}
	for _, f := range bytes.Split(raw, []byte{1}) {
		if len(f) == 0 {
			continue
		}
		kv := bytes.SplitN(f, []byte{'='}, 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("非法 FIX 字段")
		}
		tag, e := strconv.Atoi(string(kv[0]))
		if e != nil || tag <= 0 {
			return nil, fmt.Errorf("非法 FIX Tag")
		}
		ps = append(ps, Pair{tag, string(kv[1])})
	}
	return ps, nil
}
func Get(ps []Pair, tag int) string {
	for _, p := range ps {
		if p.Tag == tag {
			return p.Value
		}
	}
	return ""
}
func BodyPairs(ps []Pair) []Pair {
	out := []Pair{}
	for _, p := range ps {
		switch p.Tag {
		case 8, 9, 10, 35, 49, 56, 34, 52:
		default:
			out = append(out, p)
		}
	}
	return out
}
