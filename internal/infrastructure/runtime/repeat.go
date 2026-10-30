package runtime

import (
	"encoding/json"
	"fmt"
	"tcpconnect/internal/domain"
	"time"
)

// StartRepeat uses the same connection and serialized send lifecycle as a manual send.
func (s *Session) StartRepeat(r domain.Request, seconds int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.connected {
		return fmt.Errorf("请先连接服务")
	}
	if r.ID == "" || r.ServiceID != s.opts.Service.ID {
		return fmt.Errorf("定时发送请求与连接不匹配")
	}
	if len(r.Data) > 4<<20 || len(r.PreScript)+len(r.PostScript) > 512<<10 {
		return fmt.Errorf("请求或脚本过大")
	}
	if seconds < 1 || seconds > 3600 {
		return fmt.Errorf("定时间隔必须为1到3600秒")
	}
	if s.dictionary != nil && !r.RawMode && !s.loggedOn {
		return fmt.Errorf("FIX管理模式请先完成Logon登录，再启动定时发送")
	}
	if s.dictionary != nil && !r.RawMode {
		if _, err := r.Body(s.dictionary); err != nil {
			return err
		}
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	var snapshot domain.Request
	if err = json.Unmarshal(data, &snapshot); err != nil {
		return err
	}
	name := "request/" + r.ID
	if len(s.tasks) >= 32 && s.tasks[name] == nil {
		return fmt.Errorf("最多32个定时任务")
	}
	s.stopTask(name)
	t := &task{stop: make(chan struct{}), request: snapshot, kind: "request", intervalMS: int64(seconds) * 1000}
	s.tasks[name] = t
	previous := s.active
	s.active = snapshot
	s.emit("task", fmt.Sprintf("启动定时发送: %s，每%d秒；首次发送在一个间隔后", r.Name, seconds))
	s.active = previous
	go func() {
		ticker := time.NewTicker(time.Duration(seconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-t.stop:
				return
			case <-ticker.C:
				s.mu.Lock()
				if !s.connected || s.tasks[name] != t {
					s.mu.Unlock()
					return
				}
				err := s.sendLocked(t.request)
				if err != nil && s.tasks[name] == t {
					s.stopTask(name)
				}
				s.mu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()
	return nil
}
