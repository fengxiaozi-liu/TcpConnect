# TcpConnect 开发约定

本项目是独立的 Go + Vue TCP/FIX 桌面调试工具，使用 Wails。所有源文件使用 UTF-8。

## 分层与职责
- `internal/domain`：领域模型、仓储及运行时端口；`domain/fix` 为字典、报文和协议规则，不依赖网络、数据库或界面。
- `internal/application`：工作台用例与跨领域编排。
- `internal/infrastructure`：TCP、JavaScript、SQLite、文件和定时器实现。
- `internal/interfaces`：桌面绑定和仅本机 HTTP 调试适配。
- `frontend/src`：Vue 工作台、协议表单、脚本编辑和事件展示。

## 实施原则
TCP 通信不得依赖 FIX。FIX 字典在运行时导入，不通过生成源码实现动态表单。保留原始字节，按字节处理长度和校验和。所有同一连接的写入串行化。JavaScript VM 不得跨 goroutine 并发访问。定时任务绑定连接，断开时清理。

## 验证与交付
变更后运行匹配范围的 `go test ./...`、`go vet ./...`、前端类型检查及构建。并发运行时需要 race 测试。代码阶段完成后审查完整 diff，先修复再交付。不要把用户数据、凭据或本机数据库提交到 Git。

## 工程入口
`go run . --web` 运行本机浏览器模式；`wails dev` 运行桌面开发；`wails build` 生成 Windows 程序。详细命令及脚本契约见 README.md 与 docs。
