package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"tcpconnect/internal/application"
	"tcpconnect/internal/domain"
	"tcpconnect/internal/infrastructure/runtime"
	"tcpconnect/internal/infrastructure/storage"
	"tcpconnect/internal/interfaces"
	"time"
)

//go:embed all:frontend/dist
var frontend embed.FS

//go:embed assets/fixspec/FIX44.xml
var sampleDictionary string

func main() {
	web := flag.Bool("web", false, "运行仅本机浏览器模式")
	port := flag.Int("port", 17839, "本机浏览器端口")
	dataDir := flag.String("data-dir", "", "配置存储目录")
	flag.Parse()
	if *dataDir == "" {
		base, e := os.UserConfigDir()
		if e != nil {
			log.Fatal(e)
		}
		*dataDir = filepath.Join(base, "TcpConnect")
	}
	if e := os.MkdirAll(*dataDir, 0700); e != nil {
		log.Fatal(e)
	}
	dbPath := filepath.Join(*dataDir, "workspace.db")
	_, statErr := os.Stat(dbPath)
	freshWorkspace := os.IsNotExist(statErr)
	repo, e := storage.Open(dbPath)
	if e != nil {
		log.Fatal(e)
	}
	app, e := application.New(repo, runtime.Factory{})
	if e != nil {
		log.Fatal(e)
	}
	defer app.Close()
	if freshWorkspace {
		if e = seed(app); e != nil {
			log.Fatal(e)
		}
	}
	bridge := &interfaces.Bridge{Workbench: app}
	assets, _ := fs.Sub(frontend, "frontend/dist")
	if *web {
		listener, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
		if e != nil {
			log.Fatal(e)
		}
		server := &http.Server{Handler: bridge.Handler(assets, listener.Addr().String()), ReadHeaderTimeout: 5 * time.Second}
		fmt.Printf("TcpConnect: http://%s\n", listener.Addr())
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt)
		go func() {
			<-done
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}()
		if e = server.Serve(listener); e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
		return
	}
	e = wails.Run(&options.App{Title: "TcpConnect · TCP / FIX 工作台", Width: 1440, Height: 940, MinWidth: 1080, MinHeight: 720, AssetServer: &assetserver.Options{Assets: assets}, BackgroundColour: &options.RGBA{R: 247, G: 248, B: 252, A: 255}, Bind: []interface{}{bridge}})
	if e != nil {
		log.Fatal(e)
	}
}
func seed(app *application.Workbench) error {
	w := app.Snapshot(0).Workspace
	if len(w.Dictionaries) == 0 {
		if _, e := app.ImportDictionary("FIX44 · 示例业务字典", sampleDictionary); e != nil {
			return e
		}
		w = app.Snapshot(0).Workspace
	}
	tcpID := application.ID()
	fixID := application.ID()
	dictionaryID := w.Dictionaries[0].ID
	w.Services = append(w.Services, domain.Service{ID: tcpID, Name: "本地 TCP", Protocol: "tcp", Host: "127.0.0.1", Port: 9879, TimeoutMS: 5000, Framing: "delimiter", Delimiter: "\n"}, domain.Service{ID: fixID, Name: "FIX 4.4", Protocol: "fix", Host: "127.0.0.1", Port: 9878, TimeoutMS: 5000, DictionaryID: dictionaryID, Sender: "FixClient", Target: "Fix", Heartbeat: 30, ResetOnLogon: true, AutoLogon: false})
	w.Requests = append(w.Requests, domain.Request{ID: application.ID(), ServiceID: tcpID, Name: "Echo 回显", Data: "Hello, TCP!\n", Encoding: "text", Values: map[string]any{}}, domain.Request{ID: application.ID(), ServiceID: fixID, Name: "登录 Logon", MessageType: "A", Values: map[string]any{"EncryptMethod": "0", "HeartBtInt": "30", "ResetSeqNumFlag": "Y"}}, domain.Request{ID: application.ID(), ServiceID: fixID, Name: "新建订单", MessageType: "D", Values: map[string]any{}}, domain.Request{ID: application.ID(), ServiceID: fixID, Name: "退出 Logout", MessageType: "5", Values: map[string]any{}})
	w.Functions = append(w.Functions, domain.Function{ID: application.ID(), Name: "SOH 显示为竖线", Kind: "response", Code: `return data.replace(/\x01/g, "|");`}, domain.Function{ID: application.ID(), Name: "竖线转换为 SOH", Kind: "request", Code: `return data.replace(/\|/g, "\x01");`}, domain.Function{ID: application.ID(), Name: "按换行拆分（返回字节数）", Kind: "frame", Code: `const i = bytes.indexOf(10); return i < 0 ? 0 : i + 1;`})
	return app.Save(w)
}
