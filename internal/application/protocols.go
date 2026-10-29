package application

import (
	"fmt"
	"strings"

	"tcpconnect/internal/domain"
	"tcpconnect/internal/domain/fix"
)

// ImportProtocol persists the dictionary, service and all message templates together.
func (w *Workbench) ImportProtocol(name, xml string) (string, error) {
	d, err := fix.ParseDictionary([]byte(xml))
	if err != nil {
		return "", err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	v := clone(w.workspace)
	dictionaryID := ID()
	v.Dictionaries = append(v.Dictionaries, domain.DictionaryFile{ID: dictionaryID, Name: name, XML: xml})
	serviceName := strings.TrimSpace(strings.TrimSuffix(name, ".xml"))
	if serviceName == "" {
		serviceName = d.Version
	}
	s := domain.Service{ID: ID(), Name: serviceName, Protocol: "fix", Host: "127.0.0.1", Port: 9878, TimeoutMS: 5000, DictionaryID: dictionaryID, Sender: "FixClient", Target: "Fix", Heartbeat: 30, ResetOnLogon: true}
	v.Services = append(v.Services, s)
	appendMessages(&v, s, d)
	if err = w.commitProtocolLocked(v); err != nil {
		return "", err
	}
	return s.ID, nil
}

// SaveService creates FIX templates when adding a new service.
func (w *Workbench) SaveService(s domain.Service) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	v := clone(w.workspace)
	found := false
	for i, existing := range v.Services {
		if existing.ID == s.ID {
			v.Services[i] = s
			found = true
			break
		}
	}
	count := 0
	if !found {
		v.Services = append(v.Services, s)
		if s.Protocol == "fix" {
			d, err := workspaceDictionary(v, s.DictionaryID)
			if err != nil {
				return 0, err
			}
			count = appendMessages(&v, s, d)
		}
	}
	if err := w.commitProtocolLocked(v); err != nil {
		return 0, err
	}
	return count, nil
}

// GenerateMessages fills missing message types without changing saved cases.
func (w *Workbench) GenerateMessages(serviceID string) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	v := clone(w.workspace)
	s, err := workspaceFIXService(v, serviceID)
	if err != nil {
		return 0, err
	}
	d, err := workspaceDictionary(v, s.DictionaryID)
	if err != nil {
		return 0, err
	}
	count := appendMessages(&v, s, d)
	if err = w.commitProtocolLocked(v); err != nil {
		return 0, err
	}
	return count, nil
}

func (w *Workbench) CreateMessage(serviceID, msgType string) (domain.Request, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	v := clone(w.workspace)
	s, err := workspaceFIXService(v, serviceID)
	if err != nil {
		return domain.Request{}, err
	}
	d, err := workspaceDictionary(v, s.DictionaryID)
	if err != nil {
		return domain.Request{}, err
	}
	m, ok := d.Message(msgType)
	if !ok {
		return domain.Request{}, fmt.Errorf("消息类型未定义: %s", msgType)
	}
	r := messageRequest(s, m)
	v.Requests = append(v.Requests, r)
	if err = w.commitProtocolLocked(v); err != nil {
		return domain.Request{}, err
	}
	return r, nil
}

func workspaceFIXService(v domain.Workspace, id string) (domain.Service, error) {
	for _, s := range v.Services {
		if s.ID == id && s.Protocol == "fix" {
			return s, nil
		}
	}
	return domain.Service{}, fmt.Errorf("FIX 服务不存在")
}
func workspaceDictionary(v domain.Workspace, id string) (*fix.Dictionary, error) {
	for _, d := range v.Dictionaries {
		if d.ID == id {
			return fix.ParseDictionary([]byte(d.XML))
		}
	}
	return nil, fmt.Errorf("字典不存在")
}
func appendMessages(v *domain.Workspace, s domain.Service, d *fix.Dictionary) int {
	seen := map[string]bool{}
	for _, r := range v.Requests {
		if r.ServiceID == s.ID {
			seen[r.MessageType] = true
		}
	}
	count := 0
	for _, m := range d.Messages {
		if seen[m.Type] {
			continue
		}
		v.Requests = append(v.Requests, messageRequest(s, m))
		count++
	}
	return count
}
func messageRequest(s domain.Service, m fix.Message) domain.Request {
	values := map[string]any{}
	if m.Type == "A" {
		values["EncryptMethod"] = "0"
		values["HeartBtInt"] = fmt.Sprint(s.Heartbeat)
		if s.ResetOnLogon {
			values["ResetSeqNumFlag"] = "Y"
		}
	}
	return domain.Request{ID: ID(), ServiceID: s.ID, Name: m.Name + " (" + m.Type + ")", MessageType: m.Type, Encoding: "text", Values: values}
}
func (w *Workbench) commitProtocolLocked(v domain.Workspace) error {
	if w.closed {
		return fmt.Errorf("工作台已关闭")
	}
	if err := validateWorkspace(v); err != nil {
		return err
	}
	v.Version = 1
	if err := w.repo.Save(v); err != nil {
		return err
	}
	w.workspace = clone(v)
	return nil
}
