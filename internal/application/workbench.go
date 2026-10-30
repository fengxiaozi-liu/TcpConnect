package application

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"tcpconnect/internal/domain"
	"tcpconnect/internal/domain/fix"
	"time"
)

type Workbench struct {
	mu         sync.Mutex
	repo       domain.Repository
	factory    domain.SessionFactory
	workspace  domain.Workspace
	sessions   map[string]domain.Session
	events     []domain.Event
	nextEvent  uint64
	eventBytes int
	connecting map[string]*connectAttempt
	closed     bool
}
type connectAttempt struct{ token byte }
type State struct {
	Workspace domain.Workspace               `json:"workspace"`
	Sessions  map[string]domain.SessionState `json:"sessions"`
	Events    []domain.Event                 `json:"events"`
}

func New(repo domain.Repository, factory domain.SessionFactory) (*Workbench, error) {
	w, e := repo.Load()
	if e != nil {
		return nil, e
	}
	return &Workbench{repo: repo, factory: factory, workspace: w, sessions: map[string]domain.Session{}, connecting: map[string]*connectAttempt{}}, nil
}
func ID() string { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (w *Workbench) Emit(e domain.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.nextEvent++
	e.ID = w.nextEvent
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	w.events = append(w.events, e)
	w.eventBytes += eventSize(e)
	for len(w.events) > 1 && (len(w.events) > 500 || w.eventBytes > 16<<20) {
		old := w.events[0]
		w.eventBytes -= eventSize(old)
		w.events[0] = domain.Event{}
		w.events = w.events[1:]
	}
}
func clone(v domain.Workspace) domain.Workspace {
	b, _ := json.Marshal(v)
	var c domain.Workspace
	_ = json.Unmarshal(b, &c)
	return c
}
func (w *Workbench) Snapshot(after uint64) State {
	return w.state(after, true)
}
func (w *Workbench) Status(after uint64) State {
	return w.state(after, false)
}
func (w *Workbench) state(after uint64, includeWorkspace bool) State {
	w.mu.Lock()
	s := State{Sessions: map[string]domain.SessionState{}, Events: []domain.Event{}}
	if includeWorkspace {
		s.Workspace = clone(w.workspace)
	}
	sessions := map[string]domain.Session{}
	for id, r := range w.sessions {
		sessions[id] = r
	}
	for _, e := range w.events {
		if e.ID > after {
			s.Events = append(s.Events, e)
		}
	}
	w.mu.Unlock()
	for id, r := range sessions {
		s.Sessions[id] = r.State()
	}
	return s
}
func validateWorkspace(v domain.Workspace) error {
	if len(v.Services) > 100 || len(v.Requests) > 2000 || len(v.Functions) > 300 || len(v.Dictionaries) > 20 {
		return fmt.Errorf("工作区配置超过数量限制")
	}
	ids := map[string]bool{}
	services := map[string]bool{}
	definitions := map[string]*fix.Dictionary{}
	functions := map[string]string{}
	add := func(id string) error {
		if id == "" || ids[id] {
			return fmt.Errorf("ID 为空或重复: %s", id)
		}
		ids[id] = true
		return nil
	}
	for _, d := range v.Dictionaries {
		if e := add(d.ID); e != nil {
			return e
		}
		if _, e := fix.ParseDictionary([]byte(d.XML)); e != nil {
			return fmt.Errorf("字典 %s: %w", d.Name, e)
		}
	}
	for _, f := range v.Functions {
		if e := add(f.ID); e != nil {
			return e
		}
		if f.Kind != "request" && f.Kind != "response" && f.Kind != "frame" {
			return fmt.Errorf("未知函数类别")
		}
		if len(f.Code) > 256<<10 {
			return fmt.Errorf("函数超过 256 KiB")
		}
		functions[f.ID] = f.Kind
	}
	checkFn := func(id, kind string) bool { return id == "" || functions[id] == kind }
	for _, s := range v.Services {
		if e := add(s.ID); e != nil {
			return e
		}
		if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.Host) == "" || s.Port < 1 || s.Port > 65535 {
			return fmt.Errorf("服务名称、地址或端口无效")
		}
		if s.Protocol != "tcp" && s.Protocol != "fix" {
			return fmt.Errorf("协议必须为 tcp 或 fix")
		}
		if strings.ContainsAny(s.Host, "/\\\r\n") {
			return fmt.Errorf("主机名格式无效")
		}
		if s.Protocol == "fix" {
			d, err := serviceDictionary(v, s)
			if err != nil {
				return err
			}
			definitions[s.ID] = d
			if s.Sender == "" || s.Target == "" || strings.ContainsAny(s.Sender+s.Target, "\x01|\r\n") {
				return fmt.Errorf("Sender/Target 无效")
			}
			if s.Heartbeat < 1 || s.Heartbeat > 3600 {
				return fmt.Errorf("心跳间隔必须为 1 到 3600 秒")
			}
		}
		switch s.Framing {
		case "", "chunk", "delimiter", "fixed", "length", "custom":
		default:
			return fmt.Errorf("未知分帧方式")
		}
		if s.Framing == "fixed" && (s.FrameLength < 1 || s.FrameLength > fix.MaxFrame) {
			return fmt.Errorf("定长字节数无效")
		}
		if s.Framing == "length" && (s.FrameLength < 1 || s.FrameLength > 10) {
			return fmt.Errorf("ASCII 长度头必须为 1 到 10 字节")
		}
		if s.Framing == "custom" && s.FrameFunctionID == "" {
			return fmt.Errorf("请选择接收判断函数")
		}
		if !checkFn(s.FrameFunctionID, "frame") {
			return fmt.Errorf("接收判断函数无效")
		}
		services[s.ID] = true
	}
	for _, r := range v.Requests {
		if e := add(r.ID); e != nil {
			return e
		}
		if !services[r.ServiceID] {
			return fmt.Errorf("请求引用不存在的服务")
		}
		if !checkFn(r.RequestFunctionID, "request") || !checkFn(r.ResponseFunctionID, "response") {
			return fmt.Errorf("请求关联函数类别不匹配")
		}
		if r.PeriodSeconds < 0 || r.PeriodSeconds > 3600 {
			return fmt.Errorf("定时间隔必须为1到3600秒")
		}
		if r.MessageMode != "" && r.MessageMode != "form" && r.MessageMode != "text" {
			return fmt.Errorf("未知报文编辑模式")
		}
		if r.CustomNodes != nil && definitions[r.ServiceID] != nil {
			if _, err := r.Definition(definitions[r.ServiceID]); err != nil {
				return err
			}
		}
		if len(r.Data) > fix.MaxFrame || len(r.PreScript)+len(r.PostScript) > 512<<10 {
			return fmt.Errorf("请求或脚本过大")
		}
	}
	return nil
}
func (w *Workbench) Save(v domain.Workspace) error {
	return w.save(v, false)
}
func (w *Workbench) save(v domain.Workspace, requireIdle bool) error {
	if e := validateWorkspace(v); e != nil {
		return e
	}
	v.Version = 1
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("工作台已关闭")
	}
	if requireIdle && (len(w.sessions) > 0 || len(w.connecting) > 0) {
		return fmt.Errorf("导入替换工作区前，请断开所有连接")
	}
	serviceIDs := map[string]bool{}
	for _, s := range v.Services {
		serviceIDs[s.ID] = true
	}
	for id := range w.sessions {
		if !serviceIDs[id] {
			return fmt.Errorf("删除服务前请先断开连接")
		}
	}
	for id := range w.connecting {
		if !serviceIDs[id] {
			return fmt.Errorf("删除服务前请先取消连接")
		}
	}
	if e := w.repo.Save(v); e != nil {
		return e
	}
	w.workspace = clone(v)
	return nil
}
func (w *Workbench) ImportDictionary(name, xml string) (*fix.Dictionary, error) {
	d, e := fix.ParseDictionary([]byte(xml))
	if e != nil {
		return nil, e
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.workspace.Dictionaries) >= 20 {
		return nil, fmt.Errorf("最多保存 20 份字典")
	}
	v := clone(w.workspace)
	v.Dictionaries = append(v.Dictionaries, domain.DictionaryFile{ID: ID(), Name: name, XML: xml})
	if e = w.repo.Save(v); e != nil {
		return nil, e
	}
	w.workspace = v
	return d, nil
}
func (w *Workbench) Dictionary(id string) (*fix.Dictionary, error) {
	w.mu.Lock()
	var data string
	for _, d := range w.workspace.Dictionaries {
		if d.ID == id {
			data = d.XML
			break
		}
	}
	w.mu.Unlock()
	if data == "" {
		return nil, fmt.Errorf("字典不存在")
	}
	return fix.ParseDictionary([]byte(data))
}
func (w *Workbench) Export() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	b, e := json.MarshalIndent(w.workspace, "", "  ")
	return string(b), e
}
func (w *Workbench) Import(data string) error {
	if len(data) > 32<<20 {
		return fmt.Errorf("导入文件超过 32 MiB")
	}
	var v domain.Workspace
	if e := json.Unmarshal([]byte(data), &v); e != nil {
		return e
	}
	return w.save(v, true)
}
func (w *Workbench) Connect(id string) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return fmt.Errorf("工作台已关闭")
	}
	if w.connecting[id] != nil {
		w.mu.Unlock()
		return fmt.Errorf("正在连接")
	}
	var service domain.Service
	for _, s := range w.workspace.Services {
		if s.ID == id {
			service = s
			break
		}
	}
	if service.ID == "" {
		w.mu.Unlock()
		return fmt.Errorf("服务不存在")
	}
	old := w.sessions[id]
	attempt := &connectAttempt{}
	w.connecting[id] = attempt
	opts := domain.SessionOptions{Service: service, Functions: append([]domain.Function{}, w.workspace.Functions...), Emit: w.Emit}
	for _, d := range w.workspace.Dictionaries {
		if d.ID == service.DictionaryID {
			opts.Dictionary = d
			break
		}
	}
	w.mu.Unlock()
	if old != nil {
		old.Close()
	}
	session, e := w.factory.Connect(opts)
	w.mu.Lock()
	if w.closed || w.connecting[id] != attempt {
		w.mu.Unlock()
		if session != nil {
			session.Close()
		}
		return fmt.Errorf("连接已取消")
	}
	delete(w.connecting, id)
	if e == nil {
		w.sessions[id] = session
	} else {
		delete(w.sessions, id)
	}
	w.mu.Unlock()
	if e != nil {
		w.Emit(domain.Event{ServiceID: id, Kind: "error", Text: e.Error()})
	}
	return e
}
func (w *Workbench) Disconnect(id string) {
	w.mu.Lock()
	s := w.sessions[id]
	delete(w.sessions, id)
	delete(w.connecting, id)
	w.mu.Unlock()
	if s != nil {
		s.Close()
	}
}
func (w *Workbench) Send(r domain.Request) error {
	w.mu.Lock()
	s := w.sessions[r.ServiceID]
	w.mu.Unlock()
	if s == nil {
		return fmt.Errorf("请先连接服务")
	}
	return s.Send(r)
}
func (w *Workbench) StartRepeat(r domain.Request, seconds int) error {
	w.mu.Lock()
	s := w.sessions[r.ServiceID]
	exists := false
	for _, saved := range w.workspace.Requests {
		if saved.ID == r.ID && saved.ServiceID == r.ServiceID {
			exists = true
			break
		}
	}
	w.mu.Unlock()
	if !exists {
		return fmt.Errorf("请先保存当前请求")
	}
	if s == nil {
		return fmt.Errorf("请先连接服务")
	}
	return s.StartRepeat(r, seconds)
}
func (w *Workbench) StopTask(id, name string) {
	w.mu.Lock()
	s := w.sessions[id]
	w.mu.Unlock()
	if s != nil {
		s.StopTask(name)
	}
}
func eventSize(e domain.Event) int {
	size := len(e.Raw) + len(e.Text) + len(e.Hex) + len(e.Base64)
	if e.FIXDefinition != nil {
		b, _ := json.Marshal(e.FIXDefinition)
		size += len(b)
	}
	return size
}
func (w *Workbench) ClearRequestEvents(serviceID, requestID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	kept := make([]domain.Event, 0, len(w.events))
	w.eventBytes = 0
	for _, e := range w.events {
		if e.ServiceID == serviceID && e.RequestID == requestID {
			continue
		}
		kept = append(kept, e)
		w.eventBytes += eventSize(e)
	}
	w.events = kept
}
func (w *Workbench) ClearEvents() { w.mu.Lock(); w.events = nil; w.eventBytes = 0; w.mu.Unlock() }
func (w *Workbench) Close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	w.connecting = map[string]*connectAttempt{}
	ss := w.sessions
	w.sessions = map[string]domain.Session{}
	w.mu.Unlock()
	for _, s := range ss {
		s.Close()
	}
	_ = w.repo.Close()
}
