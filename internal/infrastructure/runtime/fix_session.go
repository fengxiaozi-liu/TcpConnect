package runtime

import (
	"fmt"
	"strconv"
	"tcpconnect/internal/domain/fix"
	"time"
)

func (s *Session) heartbeatLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		if !s.connected {
			s.mu.Unlock()
			return
		}
		interval := time.Duration(s.opts.Service.Heartbeat) * time.Second
		if interval < time.Second {
			interval = 30 * time.Second
		}
		if s.loggedOn {
			if s.awaitingTest != "" && time.Since(s.testSent) > interval {
				s.emit("error", "FIX TestRequest 超时")
				s.closeLocked()
			} else if s.awaitingTest == "" && time.Since(s.lastRead) > interval*3/2 {
				s.awaitingTest = strconv.FormatInt(time.Now().UnixMilli(), 10)
				s.testSent = time.Now()
				if e := s.sendAdmin("1", []fix.Pair{{Tag: 112, Value: s.awaitingTest}}); e != nil {
					s.emit("error", e.Error())
				}
			}
			if s.connected && time.Since(s.lastWrite) >= interval {
				if e := s.sendAdmin("0", nil); e != nil {
					s.emit("error", e.Error())
				}
			}
		}
		s.mu.Unlock()
	}
}

func (s *Session) receiveFIX(raw []byte) error {
	ps, e := fix.Pairs(raw)
	if e != nil {
		return e
	}
	if fix.Get(ps, 49) != s.opts.Service.Target || fix.Get(ps, 56) != s.opts.Service.Sender {
		return fmt.Errorf("FIX Sender/Target 与会话不匹配")
	}
	typ := fix.Get(ps, 35)
	seq, e := strconv.Atoi(fix.Get(ps, 34))
	if e != nil || seq < 1 {
		return fmt.Errorf("FIX MsgSeqNum 无效")
	}
	if typ == "A" && fix.Get(ps, 141) == "Y" {
		s.expected = 1
	}
	if seq > s.expected {
		old := s.pending[seq]
		if (len(s.pending) >= 100 && old == nil) || s.pendingBytes-len(old)+len(raw) > 16<<20 {
			return fmt.Errorf("FIX 序号缺口缓冲超过 100 条或 16 MiB")
		}
		s.pending[seq] = raw
		s.pendingBytes += len(raw) - len(old)
		if s.requestedGap != s.expected {
			s.requestedGap = s.expected
			return s.sendAdmin("2", []fix.Pair{{Tag: 7, Value: strconv.Itoa(s.expected)}, {Tag: 16, Value: "0"}})
		}
		return nil
	}
	if seq < s.expected {
		if fix.Get(ps, 43) == "Y" {
			return nil
		}
		return fmt.Errorf("FIX 序号过低: 收到 %d，期望 %d", seq, s.expected)
	}
	s.expected++
	s.requestedGap = 0
	switch typ {
	case "A":
		interval, err := strconv.Atoi(fix.Get(ps, 108))
		if err != nil || interval < 1 || interval > 3600 {
			return fmt.Errorf("对端 HeartBtInt 必须为 1 到 3600 秒")
		}
		s.opts.Service.Heartbeat = interval
		s.loggedOn = true
		s.emit("connection", "FIX 已登录，自动维护心跳与序号")
	case "0":
		if fix.Get(ps, 112) == s.awaitingTest {
			s.awaitingTest = ""
		}
	case "1":
		if id := fix.Get(ps, 112); id != "" {
			if e = s.sendAdmin("0", []fix.Pair{{Tag: 112, Value: id}}); e != nil {
				return e
			}
		}
	case "5":
		if s.loggedOn {
			_ = s.sendAdmin("5", nil)
		}
		s.emit("connection", "FIX Logout: "+fix.Get(ps, 58))
		s.closeLocked()
	case "4":
		n, e := strconv.Atoi(fix.Get(ps, 36))
		if e != nil || n < s.expected {
			return fmt.Errorf("FIX NewSeqNo 无效")
		}
		s.expected = n
	case "2":
		begin, e := strconv.Atoi(fix.Get(ps, 7))
		if e != nil || begin < 1 {
			return fmt.Errorf("ResendRequest BeginSeqNo 无效")
		}
		end, e := strconv.Atoi(fix.Get(ps, 16))
		if e != nil {
			return fmt.Errorf("ResendRequest EndSeqNo 无效")
		}
		if end == 0 || end >= s.seq {
			end = s.seq - 1
		}
		if end-begin > 1000 {
			return fmt.Errorf("重发范围超过本地 1000 条窗口")
		}
		for i := begin; i <= end; i++ {
			b, exists := s.journal[i]
			if !exists {
				return fmt.Errorf("序号 %d 已超出本地重发历史", i)
			}
			pairs, _ := fix.Pairs(b)
			mt := fix.Get(pairs, 35)
			if mt == "A" || mt == "0" || mt == "1" || mt == "2" || mt == "3" || mt == "4" || mt == "5" {
				gap := fix.Encode("4", s.opts.Service.Sender, s.opts.Service.Target, i, []fix.Pair{{Tag: 43, Value: "Y"}, {Tag: 123, Value: "Y"}, {Tag: 36, Value: strconv.Itoa(i + 1)}}, time.Now())
				if e = s.write(gap); e != nil {
					return e
				}
			} else {
				body := []fix.Pair{{Tag: 43, Value: "Y"}, {Tag: 122, Value: fix.Get(pairs, 52)}}
				body = append(body, fix.BodyPairs(pairs)...)
				if e = s.write(fix.Encode(mt, s.opts.Service.Sender, s.opts.Service.Target, i, body, time.Now())); e != nil {
					return e
				}
			}
		}
	}
	for {
		next, ok := s.pending[s.expected]
		if !ok {
			break
		}
		delete(s.pending, s.expected)
		s.pendingBytes -= len(next)
		if e = s.receiveFIX(next); e != nil {
			return e
		}
	}
	return nil
}
