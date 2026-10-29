package interfaces

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"tcpconnect/internal/application"
	"tcpconnect/internal/domain"
	"tcpconnect/internal/domain/fix"
	"time"
)

type Bridge struct{ Workbench *application.Workbench }

func (b *Bridge) Snapshot(after uint64) application.State { return b.Workbench.Snapshot(after) }
func (b *Bridge) Status(after uint64) application.State   { return b.Workbench.Status(after) }
func (b *Bridge) Save(workspace domain.Workspace) error   { return b.Workbench.Save(workspace) }
func (b *Bridge) Connect(id string) error                 { return b.Workbench.Connect(id) }
func (b *Bridge) Disconnect(id string)                    { b.Workbench.Disconnect(id) }
func (b *Bridge) Send(request domain.Request) error       { return b.Workbench.Send(request) }
func (b *Bridge) StopTask(id, name string)                { b.Workbench.StopTask(id, name) }
func (b *Bridge) ClearEvents()                            { b.Workbench.ClearEvents() }
func (b *Bridge) ImportDictionary(name, xml string) (*fix.Dictionary, error) {
	return b.Workbench.ImportDictionary(name, xml)
}
func (b *Bridge) ImportProtocol(name, xml string) (string, error) {
	return b.Workbench.ImportProtocol(name, xml)
}
func (b *Bridge) SaveService(s domain.Service) (int, error) { return b.Workbench.SaveService(s) }
func (b *Bridge) GenerateMessages(id string) (int, error)   { return b.Workbench.GenerateMessages(id) }
func (b *Bridge) CreateMessage(id, msgType string) (domain.Request, error) {
	return b.Workbench.CreateMessage(id, msgType)
}
func (b *Bridge) Dictionary(id string) (*fix.Dictionary, error) { return b.Workbench.Dictionary(id) }
func (b *Bridge) Export() (string, error)                       { return b.Workbench.Export() }
func (b *Bridge) Import(data string) error                      { return b.Workbench.Import(data) }
func (b *Bridge) Preview(r domain.Request) (string, error) {
	state := b.Workbench.Snapshot(^uint64(0))
	var svc domain.Service
	for _, s := range state.Workspace.Services {
		if s.ID == r.ServiceID {
			svc = s
			break
		}
	}
	if svc.ID == "" {
		return "", fmt.Errorf("服务不存在")
	}
	if svc.Protocol != "fix" || r.RawMode {
		return r.Data, nil
	}
	d, e := b.Workbench.Dictionary(svc.DictionaryID)
	if e != nil {
		return "", e
	}
	ps, e := d.Body(r.MessageType, r.Values)
	if e != nil {
		return "", e
	}
	seq := 1
	if s, ok := state.Sessions[svc.ID]; ok {
		seq = s.NextSeq
	}
	return strings.ReplaceAll(string(fix.Encode(r.MessageType, svc.Sender, svc.Target, seq, ps, time.Now())), fix.SOH, "|"), nil
}

// Handler exposes only named use cases. It requires same-origin JSON POSTs.
func (b *Bridge) Handler(assets fs.FS, host string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(assets)))
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "POST" || r.Host != host || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+host {
			http.Error(w, "origin denied", http.StatusForbidden)
			return
		}
		var input struct {
			Method string            `json:"method"`
			Args   []json.RawMessage `json:"args"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
		if e := json.NewDecoder(r.Body).Decode(&input); e != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		arg := func(i int, v any) error {
			if i >= len(input.Args) {
				return fmt.Errorf("缺少参数")
			}
			return json.Unmarshal(input.Args[i], v)
		}
		var result any
		var err error
		var id, name, data string
		var after uint64
		var req domain.Request
		var workspace domain.Workspace
		switch input.Method {
		case "Snapshot", "Status":
			if len(input.Args) > 0 {
				err = arg(0, &after)
			}
			if err == nil {
				if input.Method == "Status" {
					result = b.Status(after)
				} else {
					result = b.Snapshot(after)
				}
			}
		case "Save":
			if err = arg(0, &workspace); err == nil {
				err = b.Save(workspace)
			}
		case "Connect":
			if err = arg(0, &id); err == nil {
				err = b.Connect(id)
			}
		case "SaveService":
			var svc domain.Service
			if err = arg(0, &svc); err == nil {
				result, err = b.SaveService(svc)
			}
		case "GenerateMessages":
			if err = arg(0, &id); err == nil {
				result, err = b.GenerateMessages(id)
			}
		case "CreateMessage":
			if err = arg(0, &id); err == nil {
				err = arg(1, &name)
			}
			if err == nil {
				result, err = b.CreateMessage(id, name)
			}
		case "Disconnect":
			if err = arg(0, &id); err == nil {
				b.Disconnect(id)
			}
		case "Send", "Preview":
			if err = arg(0, &req); err == nil {
				if input.Method == "Send" {
					err = b.Send(req)
				} else {
					result, err = b.Preview(req)
				}
			}
		case "StopTask":
			if err = arg(0, &id); err == nil {
				err = arg(1, &name)
			}
			if err == nil {
				b.StopTask(id, name)
			}
		case "ClearEvents":
			b.ClearEvents()
		case "Dictionary":
			if err = arg(0, &id); err == nil {
				result, err = b.Dictionary(id)
			}
		case "ImportDictionary", "ImportProtocol":
			if err = arg(0, &name); err == nil {
				err = arg(1, &data)
			}
			if err == nil {
				if input.Method == "ImportProtocol" {
					result, err = b.ImportProtocol(name, data)
				} else {
					result, err = b.ImportDictionary(name, data)
				}
			}
		case "Export":
			result, err = b.Export()
		case "Import":
			if err = arg(0, &data); err == nil {
				err = b.Import(data)
			}
		default:
			err = fmt.Errorf("未知操作")
		}
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result})
	})
	return mux
}
