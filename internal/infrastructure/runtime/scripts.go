package runtime

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/dop251/goja"
	"strings"
	"time"
)

func (s *Session) initVM() error {
	vm := s.vm
	console := map[string]any{}
	for _, level := range []string{"log", "info", "warn", "error"} {
		level := level
		console[level] = func(call goja.FunctionCall) goja.Value {
			parts := []string{}
			for _, v := range call.Arguments {
				parts = append(parts, v.String())
			}
			text := strings.Join(parts, " ")
			if len(text) > 8192 {
				text = text[:8192] + "…"
			}
			s.emit("log", level+": "+text)
			return goja.Undefined()
		}
	}
	_ = vm.Set("console", console)
	variables := map[string]any{"get": func(k string) any { return s.vars[k] }, "set": func(k string, v any) {
		if _, exists := s.vars[k]; !exists && len(s.vars) >= 1000 {
			panic(vm.NewGoError(fmt.Errorf("会话变量最多 1000 个")))
		}
		s.vars[k] = v
	}, "has": func(k string) bool { _, ok := s.vars[k]; return ok }, "unset": func(k string) { delete(s.vars, k) }}
	_ = vm.Set("vars", variables)
	_ = vm.Set("pm", map[string]any{"environment": variables, "variables": variables, "globals": variables})
	_ = vm.Set("socket", map[string]any{"send": func(call goja.FunctionCall) goja.Value {
		b, e := valueBytes(call.Argument(0))
		if e == nil {
			e = s.sendData(b, s.active, !s.active.RawMode)
		}
		if e != nil {
			panic(vm.NewGoError(e))
		}
		return goja.Undefined()
	}, "disconnect": func() { s.closeLocked() }, "isConnected": func() bool { return s.connected }})
	_ = vm.Set("__utf8encode", func(text string) []int {
		b := []byte(text)
		out := make([]int, len(b))
		for i, v := range b {
			out[i] = int(v)
		}
		return out
	})
	_ = vm.Set("__utf8decode", func(v goja.Value) string {
		b, e := valueBytes(v)
		if e != nil {
			panic(vm.NewGoError(e))
		}
		return string(b)
	})
	_ = vm.Set("hex", map[string]any{"encode": func(v goja.Value) string {
		b, e := valueBytes(v)
		if e != nil {
			panic(vm.NewGoError(e))
		}
		return hex.EncodeToString(b)
	}, "decode": func(text string) []int {
		b, e := hex.DecodeString(strings.Join(strings.Fields(text), ""))
		if e != nil {
			panic(vm.NewGoError(e))
		}
		out := []int{}
		for _, v := range b {
			out = append(out, int(v))
		}
		return out
	}})
	_ = vm.Set("base64", map[string]any{"encode": func(v goja.Value) string {
		b, e := valueBytes(v)
		if e != nil {
			panic(vm.NewGoError(e))
		}
		return base64.StdEncoding.EncodeToString(b)
	}, "decode": func(text string) []int {
		b, e := base64.StdEncoding.DecodeString(text)
		if e != nil {
			panic(vm.NewGoError(e))
		}
		out := []int{}
		for _, v := range b {
			out = append(out, int(v))
		}
		return out
	}})
	register := func(call goja.FunctionCall, once bool) goja.Value {
		if !s.connected {
			panic(vm.NewGoError(fmt.Errorf("连接已断开，无法创建定时任务")))
		}
		name := call.Argument(0).String()
		ms := call.Argument(1).ToInteger()
		fn, ok := goja.AssertFunction(call.Argument(2))
		if !ok || name == "" || ms < 50 || ms > 86400000 {
			panic(vm.NewGoError(fmt.Errorf("定时器需要名称、50..86400000 毫秒、回调函数")))
		}
		if len(s.tasks) >= 32 && s.tasks[name] == nil {
			panic(vm.NewGoError(fmt.Errorf("最多 32 个定时任务")))
		}
		s.stopTask(name)
		t := &task{stop: make(chan struct{}), request: s.active}
		s.tasks[name] = t
		s.emit("task", fmt.Sprintf("启动定时任务 %s (%dms)", name, ms))
		go func() {
			timer := time.NewTicker(time.Duration(ms) * time.Millisecond)
			defer timer.Stop()
			for {
				select {
				case <-t.stop:
					return
				case <-timer.C:
					s.mu.Lock()
					if !s.connected || s.tasks[name] != t {
						s.mu.Unlock()
						return
					}
					previous := s.active
					s.active = t.request
					e := s.run(func() error { _, err := fn(goja.Undefined()); return err })
					s.active = previous
					if e != nil {
						s.emit("error", "定时任务 "+name+": "+e.Error())
						s.stopTask(name)
					}
					if once {
						s.stopTask(name)
					}
					s.mu.Unlock()
					if once || e != nil {
						return
					}
				}
			}
		}()
		return vm.ToValue(name)
	}
	_ = vm.Set("timers", map[string]any{"setInterval": func(c goja.FunctionCall) goja.Value { return register(c, false) }, "setTimeout": func(c goja.FunctionCall) goja.Value { return register(c, true) }, "clear": func(name string) { s.stopTask(name) }})
	_, e := vm.RunString(`class TextEncoder { encode(text) { return new Uint8Array(__utf8encode(String(text))); } } class TextDecoder { decode(bytes) { return __utf8decode(bytes); } } function setInterval(fn,ms){ const name="interval-"+Date.now()+"-"+Math.random(); return timers.setInterval(name,ms,fn); } function setTimeout(fn,ms){ const name="timeout-"+Date.now()+"-"+Math.random(); return timers.setTimeout(name,ms,fn); } function clearInterval(name){ timers.clear(name); } function clearTimeout(name){ timers.clear(name); }`)
	return e
}
