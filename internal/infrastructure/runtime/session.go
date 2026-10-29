package runtime

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/dop251/goja"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"tcpconnect/internal/domain"
	"tcpconnect/internal/domain/fix"
	"time"
)

type Factory struct{}
type task struct {
	stop    chan struct{}
	request domain.Request
}
type Session struct {
	mu                  sync.Mutex
	conn                net.Conn
	opts                domain.SessionOptions
	vm                  *goja.Runtime
	functions           map[string]domain.Function
	dictionary          *fix.Dictionary
	connected, loggedOn bool
	seq, expected       int
	tasks               map[string]*task
	vars                map[string]any
	active              domain.Request
	lastWrite, lastRead time.Time
	journal             map[int][]byte
	journalBytes        int
	pending             map[int][]byte
	pendingBytes        int
	requestedGap        int
	awaitingTest        string
	testSent            time.Time
	sendDepth           int
}

func (Factory) Connect(opts domain.SessionOptions) (domain.Session, error) {
	timeout := time.Duration(opts.Service.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if timeout > 30*time.Second {
		timeout = 30 * time.Second
	}
	var d *fix.Dictionary
	var e error
	if opts.Service.Protocol == "fix" {
		d, e = fix.ParseDictionary([]byte(opts.Dictionary.XML))
		if e != nil {
			return nil, e
		}
	}
	c, e := net.DialTimeout("tcp", net.JoinHostPort(opts.Service.Host, strconv.Itoa(opts.Service.Port)), timeout)
	if e != nil {
		return nil, fmt.Errorf("连接失败: %w", e)
	}
	s := &Session{conn: c, opts: opts, vm: goja.New(), functions: map[string]domain.Function{}, dictionary: d, connected: true, seq: 1, expected: 1, tasks: map[string]*task{}, vars: map[string]any{}, lastWrite: time.Now(), lastRead: time.Now(), journal: map[int][]byte{}, pending: map[int][]byte{}}
	for _, f := range opts.Functions {
		s.functions[f.ID] = f
	}
	if e = s.initVM(); e != nil {
		c.Close()
		return nil, e
	}
	s.emit("connection", "TCP 已连接 "+c.RemoteAddr().String())
	if d != nil && opts.Service.AutoLogon {
		body := []fix.Pair{{Tag: 98, Value: "0"}, {Tag: 108, Value: strconv.Itoa(opts.Service.Heartbeat)}}
		if opts.Service.ResetOnLogon {
			body = append(body, fix.Pair{Tag: 141, Value: "Y"})
		}
		if opts.Service.Username != "" {
			body = append(body, fix.Pair{Tag: 553, Value: opts.Service.Username})
		}
		if opts.Service.Password != "" {
			body = append(body, fix.Pair{Tag: 554, Value: opts.Service.Password})
		}
		if e = s.sendAdmin("A", body); e != nil {
			s.closeLocked()
			return nil, e
		}
	}
	go s.readLoop()
	if d != nil {
		go s.heartbeatLoop()
	}
	return s, nil
}
func (s *Session) emit(kind, text string) {
	s.opts.Emit(domain.Event{ServiceID: s.opts.Service.ID, Kind: kind, Text: text})
}
func (s *Session) record(direction string, b []byte, text string) {
	s.opts.Emit(domain.Event{ServiceID: s.opts.Service.ID, Kind: "message", Direction: direction, Text: text, Raw: string(b), Hex: hex.EncodeToString(b), Base64: base64.StdEncoding.EncodeToString(b), Size: len(b)})
}
func (s *Session) State() domain.SessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks := []string{}
	for n := range s.tasks {
		tasks = append(tasks, n)
	}
	sort.Strings(tasks)
	return domain.SessionState{Connected: s.connected, LoggedOn: s.loggedOn, NextSeq: s.seq, ExpectedSeq: s.expected, Tasks: tasks}
}
func (s *Session) Close() { s.mu.Lock(); defer s.mu.Unlock(); s.closeLocked() }
func (s *Session) closeLocked() {
	if !s.connected {
		return
	}
	s.connected = false
	s.loggedOn = false
	_ = s.conn.Close()
	for n, t := range s.tasks {
		close(t.stop)
		delete(s.tasks, n)
	}
	s.emit("connection", "TCP 已断开，定时任务已停止")
}
func (s *Session) StopTask(name string) { s.mu.Lock(); defer s.mu.Unlock(); s.stopTask(name) }
func (s *Session) stopTask(name string) {
	if t, ok := s.tasks[name]; ok {
		close(t.stop)
		delete(s.tasks, name)
		s.emit("task", "停止定时任务: "+name)
	}
}
func (s *Session) run(work func() error) error {
	s.vm.ClearInterrupt()
	done := make(chan struct{})
	timer := time.AfterFunc(1500*time.Millisecond, func() { s.vm.Interrupt("脚本执行超过 1500ms"); close(done) })
	err := work()
	if !timer.Stop() {
		<-done
	}
	s.vm.ClearInterrupt()
	return err
}
func (s *Session) call(code string, data []byte) (goja.Value, error) {
	if code == "" {
		return s.vm.ToValue(string(data)), nil
	}
	if len(code) > 256<<10 {
		return nil, fmt.Errorf("脚本超过 256 KiB")
	}
	fn, e := s.vm.RunString("(function(data, bytes) {\n" + code + "\n})")
	if e != nil {
		return nil, e
	}
	f, ok := goja.AssertFunction(fn)
	if !ok {
		return nil, fmt.Errorf("脚本不是函数")
	}
	ints := make([]int, len(data))
	for i, b := range data {
		ints[i] = int(b)
	}
	return f(goja.Undefined(), s.vm.ToValue(string(data)), s.vm.ToValue(ints))
}
func (s *Session) transform(id string, b []byte) ([]byte, error) {
	if id == "" {
		return b, nil
	}
	f, ok := s.functions[id]
	if !ok {
		return nil, fmt.Errorf("处理函数不存在")
	}
	v, e := s.call(f.Code, b)
	if e != nil {
		return nil, e
	}
	if goja.IsUndefined(v) || goja.IsNull(v) {
		return nil, fmt.Errorf("处理函数必须返回字符串或字节数组")
	}
	return valueBytes(v)
}
func valueBytes(v goja.Value) ([]byte, error) {
	switch x := v.Export().(type) {
	case string:
		return []byte(x), nil
	case []uint8:
		return x, nil
	case []int64:
		b := make([]byte, len(x))
		for i, n := range x {
			if n < 0 || n > 255 {
				return nil, fmt.Errorf("字节超出 0..255")
			}
			b[i] = byte(n)
		}
		return b, nil
	case []interface{}:
		b := make([]byte, len(x))
		for i, n := range x {
			var value int64
			switch y := n.(type) {
			case int64:
				value = y
			case int32:
				value = int64(y)
			case float64:
				if y != float64(int64(y)) {
					return nil, fmt.Errorf("字节必须为整数")
				}
				value = int64(y)
			default:
				return nil, fmt.Errorf("非法字节")
			}
			if value < 0 || value > 255 {
				return nil, fmt.Errorf("字节超出 0..255")
			}
			b[i] = byte(value)
		}
		return b, nil
	}
	return nil, fmt.Errorf("数据必须为字符串或 Uint8Array/字节数组")
}
func (s *Session) Send(r domain.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.connected {
		return fmt.Errorf("连接已断开")
	}
	s.active = r
	err := s.run(func() error {
		var b []byte
		var e error
		if s.dictionary != nil && !r.RawMode {
			ps, err := s.dictionary.Body(r.MessageType, r.Values)
			if err != nil {
				return err
			}
			b = fix.Encode(r.MessageType, s.opts.Service.Sender, s.opts.Service.Target, s.seq, ps, time.Now())
		} else {
			switch r.Encoding {
			case "hex":
				b, e = hex.DecodeString(strings.Join(strings.Fields(r.Data), ""))
			case "base64":
				b, e = base64.StdEncoding.DecodeString(r.Data)
			default:
				b = []byte(r.Data)
			}
		}
		if e != nil {
			return fmt.Errorf("报文解码: %w", e)
		}
		if r.PreScript != "" {
			v, err := s.call(r.PreScript, b)
			if err != nil {
				return fmt.Errorf("前置脚本: %w", err)
			}
			if !goja.IsUndefined(v) {
				b, err = valueBytes(v)
				if err != nil {
					return err
				}
			}
		}
		return s.sendData(b, r, !r.RawMode)
	})
	if err != nil {
		s.emit("error", err.Error())
	}
	return err
}
func (s *Session) sendData(b []byte, r domain.Request, managed bool) error {
	if s.sendDepth >= 4 {
		return fmt.Errorf("脚本递归发送超过限制")
	}
	s.sendDepth++
	defer func() { s.sendDepth-- }()
	var e error
	b, e = s.transform(r.RequestFunctionID, b)
	if e != nil {
		return fmt.Errorf("请求处理函数: %w", e)
	}
	if s.dictionary != nil && managed {
		if !bytes.Contains(b, []byte(fix.SOH)) {
			b = []byte(strings.ReplaceAll(string(b), "|", fix.SOH))
		}
		ps, err := fix.Pairs(b)
		if err != nil {
			return err
		}
		typ := fix.Get(ps, 35)
		if typ == "" {
			return fmt.Errorf("FIX 缺少 MsgType(35)")
		}
		if typ == "A" {
			interval, err := strconv.Atoi(fix.Get(ps, 108))
			if err != nil || interval < 1 || interval > 3600 {
				return fmt.Errorf("HeartBtInt 必须为 1 到 3600 秒")
			}
			s.opts.Service.Heartbeat = interval
		}
		if typ == "A" && fix.Get(ps, 141) == "Y" {
			s.seq = 1
			s.expected = 1
			s.journal = map[int][]byte{}
			s.journalBytes = 0
			s.pending = map[int][]byte{}
			s.pendingBytes = 0
			s.requestedGap = 0
		}
		b = fix.Encode(typ, s.opts.Service.Sender, s.opts.Service.Target, s.seq, fix.BodyPairs(ps), time.Now())
		if e = s.write(b); e != nil {
			return e
		}
		s.remember(s.seq, b)
		s.seq++
		return nil
	}
	return s.write(b)
}
func (s *Session) write(b []byte) error {
	if !s.connected {
		return fmt.Errorf("连接已断开")
	}
	if len(b) == 0 || len(b) > fix.MaxFrame {
		return fmt.Errorf("发送数据必须为 1 字节到 4 MiB")
	}
	_ = s.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	left := b
	for len(left) > 0 {
		n, e := s.conn.Write(left)
		if e != nil {
			s.closeLocked()
			return fmt.Errorf("发送失败: %w", e)
		}
		if n == 0 {
			s.closeLocked()
			return fmt.Errorf("发送未写入数据")
		}
		left = left[n:]
	}
	s.lastWrite = time.Now()
	s.record("send", b, string(b))
	return nil
}
func (s *Session) remember(seq int, b []byte) {
	s.journal[seq] = bytes.Clone(b)
	s.journalBytes += len(b)
	if old := s.journal[seq-1000]; old != nil {
		s.journalBytes -= len(old)
		delete(s.journal, seq-1000)
	}
	for s.journalBytes > 16<<20 {
		oldest := seq
		for n := range s.journal {
			if n < oldest {
				oldest = n
			}
		}
		s.journalBytes -= len(s.journal[oldest])
		delete(s.journal, oldest)
	}
}
func (s *Session) sendAdmin(typ string, body []fix.Pair) error {
	b := fix.Encode(typ, s.opts.Service.Sender, s.opts.Service.Target, s.seq, body, time.Now())
	if e := s.write(b); e != nil {
		return e
	}
	s.remember(s.seq, b)
	s.seq++
	return nil
}
func (s *Session) readLoop() {
	buf := make([]byte, 32<<10)
	pending := []byte{}
	for {
		n, readErr := s.conn.Read(buf)
		s.mu.Lock()
		if !s.connected {
			s.mu.Unlock()
			return
		}
		if n > 0 {
			pending = append(pending, buf[:n]...)
			s.lastRead = time.Now()
			if len(pending) > fix.MaxFrame {
				s.emit("error", "接收缓冲超过 4 MiB，断开连接")
				s.closeLocked()
				s.mu.Unlock()
				return
			}
			err := s.run(func() error {
				for len(pending) > 0 {
					count, e := s.frameLength(pending)
					if e != nil {
						return e
					}
					if count == 0 {
						break
					}
					if count < 0 || count > len(pending) {
						return fmt.Errorf("接收函数返回的字节数无效")
					}
					frame := bytes.Clone(pending[:count])
					pending = pending[count:]
					if e = s.receive(frame); e != nil {
						return e
					}
				}
				return nil
			})
			if err != nil {
				s.emit("error", err.Error())
				s.closeLocked()
				s.mu.Unlock()
				return
			}
		}
		if readErr != nil {
			if len(pending) > 0 {
				s.record("receive", pending, string(pending))
				s.emit("error", fmt.Sprintf("连接关闭时尚有 %d 字节未组成完整报文", len(pending)))
			}
			s.emit("connection", "对端关闭连接: "+readErr.Error())
			s.closeLocked()
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
	}
}
func (s *Session) frameLength(b []byte) (int, error) {
	if s.dictionary != nil {
		return fix.FrameLength(b)
	}
	switch s.opts.Service.Framing {
	case "delimiter":
		sep := []byte(s.opts.Service.Delimiter)
		if len(sep) == 0 {
			return 0, fmt.Errorf("分隔符不能为空")
		}
		if i := bytes.Index(b, sep); i >= 0 {
			return i + len(sep), nil
		}
		return 0, nil
	case "fixed":
		if len(b) >= s.opts.Service.FrameLength {
			return s.opts.Service.FrameLength, nil
		}
		return 0, nil
	case "length":
		h := s.opts.Service.FrameLength
		if h < 1 || h > 10 {
			return 0, fmt.Errorf("长度头无效")
		}
		if len(b) < h {
			return 0, nil
		}
		n, e := strconv.Atoi(string(b[:h]))
		if e != nil || n < 0 || n > fix.MaxFrame-h {
			return 0, fmt.Errorf("ASCII 长度头无效")
		}
		if len(b) < h+n {
			return 0, nil
		}
		return h + n, nil
	case "custom":
		f, ok := s.functions[s.opts.Service.FrameFunctionID]
		if !ok {
			return 0, fmt.Errorf("接收判断函数不存在")
		}
		v, e := s.call(f.Code, b)
		if e != nil {
			return 0, e
		}
		if v.ExportType() != nil && v.ExportType().Kind().String() == "bool" {
			if v.ToBoolean() {
				return len(b), nil
			}
			return 0, nil
		}
		n := v.ToFloat()
		if n != float64(int(n)) {
			return 0, fmt.Errorf("接收判断函数必须返回整数")
		}
		return int(n), nil
	default:
		return len(b), nil
	}
}
func (s *Session) receive(b []byte) error {
	r := s.active
	shown, e := s.transform(r.ResponseFunctionID, b)
	if e != nil {
		s.emit("error", "返回处理函数: "+e.Error())
		shown = b
	}
	s.record("receive", b, string(shown))
	if s.dictionary != nil {
		if e = s.receiveFIX(b); e != nil {
			return e
		}
	}
	if r.PostScript != "" {
		if _, e = s.call(r.PostScript, b); e != nil {
			s.emit("error", "后置脚本: "+e.Error())
		}
	}
	return nil
}
