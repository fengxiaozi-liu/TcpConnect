# Review

## 全局问题列表

### 未解决问题

- 无。

### 已解决问题

- [✅️] REVIEW-001 [P1 | 正确性 | application/workbench.go] 断开或关闭期间完成的连接可能重新进入会话表。连接尝试使用独立身份，取消后的迟到连接立即关闭；增加阻塞工厂回归测试。
- [✅️] REVIEW-002 [P2 | 正确性 | domain/fix/dictionary.go] 启用可选组件时可能漏校验必填子字段，重复组可能缺少分隔首字段。校验组件启用标志及每条组记录的首字段，补充回归。
- [✅️] REVIEW-003 [P1 | 正确性 | runtime/session.go] 返回展示函数抛错会跳过 FIX 会话处理与后置脚本。展示错误回退到原始数据，继续协议与后置处理；增加回归测试。
- [✅️] REVIEW-004 [P2 | 性能 | App.vue / decode.ts / fix_session.go] 前端历史、序号缺口载荷缺少容量上限，字段解码重复遍历。增加内存预算，解码改为顺序消费，验证嵌套组路径。
- [✅️] REVIEW-005 [P2 | 正确性 | CodeEditor.vue] 新版 Monaco 的 EditContext 与自动输入不兼容。显式使用 textarea 输入路径，验证编辑、发送保存与刷新恢复。
- [✅️] REVIEW-006 [P2 | 正确性 | FixFields.vue / codec.go] 多值枚举只能选单值。支持多选及按空格序列化，对每个枚举值校验。
- [✅️] REVIEW-007 [P2 | 正确性 | runtime/session.go / fix_session.go] 手动 Logon 的心跳、重置行为可能与表单不一致。发送按实际 108/141 处理，登录确认采用对端 108；重发/GAPFILL 和空闲心跳均有真实字节流测试。
- [✅️] REVIEW-008 [P2 | 正确性 | main.go] 删除全部服务后重启可能重新创建示例。仅新建数据库时初始化示例。
- [✅️] REVIEW-009 [P2 | 安全 | frontend/package.json] Monaco 的传递 DOMPurify 版本存在已知漏洞。通过 override 更新，最终完整 npm audit 为 0 漏洞。
- [✅️] REVIEW-010 [P2 | 完成度 | App.vue] 通用 TCP 服务空页面误展示 FIX XML 导入入口。按钮现在仅在 FIX 服务页面显示；协议页、FIX 设置和独立的新建菜单仍提供字典导入。
- [✅️] REVIEW-011 [P2 | 完成度 | application/protocols.go / App.vue] 导入字典后没有服务消息树，新建 FIX 请求统一为未命名 Logon。增加原子导入建服务及全消息模板、已有服务补全、类型选择创建和命名，实际 FIX44.xml 生成 181 个模板。
- [✅️] REVIEW-012 [P2 | 正确性 | App.vue] 删除入口缺失，原删除方法依赖当前选中项，保存失败前已移除界面记录。左侧添加独立确认入口，按显式 ID 构造候选工作区，保存成功后更新标签及选中状态，失败保留记录。
- [✅️] REVIEW-013 [P2 | 正确性 | App.vue] 切换消息时旧预览仍保留，迟到响应可能覆盖新消息预览。切换与编辑时清空预览并使用修订号忽略过期响应，打开消息同时展开并滚动到选中项。

- [✅️] REVIEW-014 [P2 | 完成度 | App.vue] 接收规则只展示抽象的“自定义判断函数”，实际绑定入口隐藏。规则下拉框直接列出 frame 类函数并按 ID 绑定；提供管理往返、草稿保留和绑定提示，切换内置规则解除关联；保持已选函数，避免类型切换监听器误选首个函数。

- [✅️] REVIEW-015 [P2 | 正确性 | App.vue] 搜索只过滤服务，服务下仍渲染全部消息。计算过滤后的消息视图并渲染匹配项，支持消息类型、大小写/空格规范化、自动展开、无结果和清除；全量数据仍用于保存、模板和删除计数。

- [✅️] REVIEW-016 [P2 | 完成度 | protocols.go / Bridge / MessagePicker.vue / App.vue] XML 导入和新建服务无接口选择，直接生成全部模板。改为只读预览、默认空选、按明确类型确认；补充模板复用同一选择窗口，完整字典保留，空/无效选择后端拒绝，重复类型去重。已有项禁选且通过 aria-disabled 声明，成功预览清除此前 XML 错误提示。

## 审查模式与上下文


- 模式：Feature review。
- 范围：空 Git 仓库上的全部新增应用、前后端、字典、构建配置、测试及文档；包括完整交付内容，非仅最后一次局部修改。
- 输入：spec.md、plan.md、tasks.md、AGENTS.md、各阶段审查和实际源文件。
- 已加载：speckit-code-review 及报告模板；项目为独立 Wails 桌面应用，依据本项目 AGENTS.md，不套用原 FIXService 的 Kratos 生成链路。

## 项目知识与规范依据

DDD 领域不依赖界面、SQL 或网络实现；TCP 公共通信不加载 FIX 字典；接口适配调用同一应用层。Goja 单线程访问、连接写入串行、原始字节保留、任务绑定连接。源码 UTF-8，生成前端产物由构建产生。

## 完成度

- 结论：通过。
- 覆盖说明：TCP 长连接、前后置脚本、三类函数、定时发送/停止、可见日志、本地配置和导入导出均落地；独立 FIX 字典页及动态字段/组件/重复组表单、会话和字段解析均落地。提供 Windows EXE、使用/架构/脚本/验收文档和本机模拟对端。
- 待修复问题：无。

## 正确性

- 结论：通过。
- 覆盖说明：Go 测试验证中文/二进制字节、分包粘包、残包、自定义分帧、脚本中断恢复、定时任务停止、FIX 登录/TestRequest/重发/GAPFILL/空闲心跳、取消连接。E2E 验证实际 UI 编辑与配置恢复、同连接发送、FIX 登录与字典导入，并从 XML 表单发送含重复组的 NewOrderSingle。Windows 原生窗口验证 Go 绑定和 TCP 回显。
- 待修复问题：无。

## 架构与规范

- 结论：通过。
- 覆盖说明：domain 定义规则与端口，application 编排，infrastructure 实现 TCP/Goja/SQLite，interfaces 适配 Wails/loopback HTTP，Vue 通过统一 API 调用。不修改原 FIXService 工程。编解码按字节处理，原始与展示数据独立。
- 待修复问题：无。

## 性能

- 结论：通过本地调试工具的验收范围。
- 覆盖说明：事件、重发、缺口、XML、分帧、定时任务均设上限；轮询状态不传输完整 XML；避免应用锁与会话锁相互等待；字段解码按序消费。前端包含 Monaco，主包约 3.19 MB，Vite 产生体积提示；桌面从本地嵌入资源加载，首屏已实测可用，未进行吞吐基准或长时间压力测试。
- 待修复问题：无。

## 安全

- 结论：通过本地可信脚本的约定范围。
- 覆盖说明：HTTP 绑定 loopback，校验 Host/Origin/JSON POST；SQL 参数化；XML 不支持外部实体并限制输入与递归；界面使用文本渲染。脚本无文件/系统命令/任意 HTTP API，并设执行超时。凭据本机明文保存和导出行为在界面/文档明确。完整 npm audit 为 0 漏洞。
- 待修复问题：无。

## 验证记录

2026-10-29，Windows/amd64：

- `go test ./...`、`go test -race ./internal/...`、`go vet ./...` 通过。
- 前端 3 个单元测试、TypeScript 类型检查、Vite 生产构建通过。
- Playwright 3 个完整场景通过，含 TCP 脚本/定时器/配置恢复、FIX 字典/登录/TestRequest/订单/重复组，以及自定义模板生成/补全/创建/删除/失败保存/刷新恢复。
- Wails v2.11.0 构建 `build/bin/TcpConnect.exe` 成功；前端先单独生产构建，再使用 `wails build -s -skipbindings` 打包。
- 原生 WebView2 窗口正常呈现并通过 TCP 连接/发送/回显/断开；浏览器与桌面适配调用同一 Bridge。

## 假设与未知项

- 当前字典和会话支持 FIX 4.4，序号与重发日志只在当前连接内维护，非生产交易网关。
- DATA 字段中内嵌 SOH 的二进制扩展不在当前字段解析范围，任意字节可在通用 TCP 模式测试。
- 业务条件和请求响应配对无法仅由 QuickFIX XML 推导，留给用例与脚本。
- 用户真实 FIX 服务尚待用户验收，现有网络验证使用本机模拟对端。
- 脚本供可信本地用户使用；执行超时不等于独立进程内存隔离。

## 结论与下一步

审查通过，已发现问题均修复并回归。可直接打开 Windows 程序，按 docs/acceptance.md 对接真实服务验收。

## 验收修正复审：TCP 与 FIX 入口

范围：App.vue 的服务空页面、导入入口及引导文案。依据用户要求的通用 TCP / 独立 FIX 页面边界与 AGENTS.md，使用 speckit-code-review 流程复审本轮完整变更。

- 完成度：通过。TCP 页面移除 FIX XML 按钮，保留新建请求；FIX 服务仍能导入。
- 正确性：通过。条件来自当前服务 protocol，协议库及导入处理保持可达；空页面文案只引导打开或新建请求。
- 架构与规范：通过。本轮仅调整界面展示，不改变通信、存储或字典领域。
- 性能、安全：通过。新增常量条件判断，不引入网络调用或新的输入面。
- 验证：TypeScript/Vite 生产构建、现有 TCP 与 FIX 两个 Playwright 场景通过；Windows 更新程序构建成功。
- 交付：旧程序正在运行，更新文件为 `build/bin/TcpConnect-updated.exe`。关闭旧窗口后打开更新文件，沿用原本的配置目录。

## 验收修正复审：消息模板与删除

模式：Feature review，使用 speckit-code-review，依据本项目 AGENTS.md 和 Phase 4 的 spec/plan/tasks。项目为独立 Wails + Vue 应用，没有对应 Kratos 项目适配 skill，不引入原 FIXService 的生成链路。审查本轮完整变更：protocols.go 及测试、Bridge 原生/HTTP 入口、App.vue、服务树样式、三个 E2E 场景和相关文档；已有文件与 `.local/phase4-before` 快照逐项比较。

- 完成度：通过。XML 自动创建服务及所有消息；新服务生成、旧服务补全、指定类型新建、侧栏服务/消息删除均可达。README 和验收文档说明用法，模板不推测业务数据。
- 正确性：通过。实际 XML 181 种消息逐类型生成，无 Logon 的自定义 XML 也能生成组件字段表单；重复补全不覆盖已有用例。无效类型和存储失败保持工作区原样，SQLite 重开保持结果。删除目标通过显式 ID 传入，失败不移除记录，删除非活动项不干扰当前消息。预览清空并忽略迟到响应。
- 架构与规范：通过。模板编排在 application，字段和 XML 规则仍在 domain/fix，Bridge 两种适配复用同一用例；未修改 FIXService。服务删除先断开以清理任务，保存失败时记录仍在，但连接需手动重连。
- 性能：通过本地工具范围。按消息类型建立集合补全，保存前继续使用 100 服务/2000 请求/20 字典等上限。181 模板的导入、表单及删除已实际验证；沿用 Monaco 包体提示，不影响构建。
- 安全：通过。新增入口复用原有 Host/Origin/JSON 检查与字典/引用/数量校验，不引入外部传输。删除均有确认，用户真实配置未用于自动删除测试。
- 验证：`go test ./...`、`go test -race ./internal/...`、`go vet ./...` 通过；前端 3 个单元测试、类型检查与生产构建通过；Playwright 3 场景通过（19.3 秒）。UTF-8/尾随空白及 `git diff --check` 通过。
- 桌面与交付：Wails 构建成功，`build/bin/TcpConnect.exe` 和 `TcpConnect-updated.exe` 均已更新。新版原生 WebView2 窗口确认删除/补全入口及 FIX 类型选择对话框可用；创建和删除的完整持久化验证使用隔离浏览器测试工作区。原生检查取消弹窗，未改动用户配置。
- 结论：通过。无未解决 finding。真实业务字段内容及对端规则继续由用户验收。

## 验收修正复审：接收规则绑定自定义函数

模式：Feature review，沿用 speckit-code-review 及本项目 Wails/DDD AGENTS.md。审查 Phase 5 的 spec/plan/tasks、App.vue 与修改前 `.local/phase5-before` 的完整差异、新 E2E 场景、README/架构/脚本文档/验收说明；同时核实已有应用校验与运行时按 FrameFunctionID 执行的路径。无适用于本独立项目的 Kratos skill，不修改原 FIXService。

- 完成度：通过。接收规则直接列出已建函数，管理/绑定往返服务草稿、当前绑定名称、保存/重连说明均可见。
- 正确性：通过。仅 frame 类函数可选，ID 映射兼容旧配置；切换内置规则清除关联。函数类型监听保留有效选中项，避免返回编辑时选错函数。E2E 从新建服务进入函数管理、创建和绑定、切换规则、保存/刷新/再次编辑，随后实际连接验证分包/粘包/残包三帧，不依赖 TCP 块边界。
- 架构与规范：通过。仅界面编排和用例验收变更，沿用应用层校验、持久化和运行时端口；不引入领域到界面的依赖、不改变活动连接生命周期。
- 性能：通过。选项由现有函数列表计算，使用既有工作区函数数量上限，无额外轮询或运行时开销。
- 安全：通过。仅处理本地函数 ID 和既有可信脚本，不新增网络入口、权限或外部数据传输。
- 验证：Go 全包测试及 go vet 通过；TypeScript/Vite 构建通过（既有 Monaco 体积提示）；Playwright 4 个场景通过（28.4 秒），含新增真实 TCP 自定义函数分帧。本轮无并发运行时代码修改。
- 交付：`wails build -s -skipbindings -o TcpConnect-updated.exe` 成功。当前用户正在运行 canonical 程序，保留其进程与配置，交付 `build/bin/TcpConnect-updated.exe`；关闭旧窗口后打开更新程序，保存服务并重新连接使规则生效。
- 结论：通过，无未解决 finding。完整新流程使用隔离浏览器工作区验证，未改动用户服务绑定。

## 验收修正复审：消息树搜索

模式：Feature review，使用既有 speckit-code-review 流程及本项目 AGENTS.md。对比 `.local/phase6-before` 审查 App.vue 完整差异，并核查新增 Playwright 搜索场景、README/验收说明及 Phase 6 需求和任务。项目为独立 Wails 应用，无适配的 Kratos skill。

- 完成度：通过。服务/消息名及消息类型搜索、规范化关键词、自动展开、结果计数、无结果提示和清除入口已实现。
- 正确性：通过。搜索 Logon 只显示匹配消息，服务名命中展示该服务全部消息；空白搜索等同清空，已有编辑和选中内容保留。过滤视图不修改原始工作区，保存与删除计数仍使用全量数据，搜索中保存后重启保持 181 个模板。
- 架构与规范：通过。仅 Vue 展示及验收文档变更，复用原有数据结构，不改变应用、存储、TCP/FIX 运行时及原 FIXService。
- 性能：通过。过滤结果使用 computed 缓存，仅相关状态变化时重算；受已有工作区数量限制，实际 181 消息字典的输入和恢复已验证。
- 安全：通过。搜索只在内存匹配文本，使用 Vue 文本渲染；没有新增网络传输、脚本执行或持久数据操作。
- 验证：Go 全包测试、go vet、前端 3 个单元测试、TypeScript/Vite 构建通过；Playwright 5 场景通过（32.3 秒），新场景覆盖收起后的自动展开、大小写/空格、消息类型、无结果、清除与保存刷新。截图 `.local/screenshots/message-search.png` 已检查。
- 交付：Wails Windows 构建成功，`build/bin/TcpConnect.exe` 及 `TcpConnect-updated.exe` 更新；未终止用户进程或改动用户配置。构建仍有既有 Monaco 包体提示。
- 结论：通过，无未解决 finding。

## 验收修正复审：按选择导入接口

模式：Feature review，继续使用 speckit-code-review，依据独立 Wails/DDD 项目的 AGENTS.md。审查 Phase 7 的 spec/plan/tasks、完整前后端差异、全部新增 MessagePicker、应用回归和 E2E、README/架构/验收文档；已有代码与 `.local/phase7-before` 对比。此项目无对应 Kratos 专属适配，不修改原 FIXService。

- 完成度：通过。XML 解析后先选择接口；默认未选，支持名称/类型搜索、跨搜索多选、全选当前结果、清空及数量展示。导入、新字典服务和既有服务补充均按选中类型创建；取消新服务选择返回原配置草稿。
- 正确性：通过。PreviewDictionary 不接触仓储，确认前取消或 XML 错误不新增数据。应用层要求至少一项、校验每个类型并去重；完整字典、服务及选中模板一次保存，失败无半成品。已有模板保留，旧工作区结构兼容。导入失败保留选择可重试，成功预览不再显示前一次 XML 错误。禁选项使用实际 disabled 及显式 aria-disabled。
- 架构与规范：通过。领域 XML 解析独立，消息选择规则由应用层执行；Wails/HTTP Bridge 同步传递类型数组，Vue 三个入口复用选择组件，无领域/基础设施依赖界面。
- 性能：通过本次 181 消息的本地联调范围。过滤使用 computed 和集合去重，XML、工作区数量上限继续生效；不增加后台轮询。未进行超大字典界面压力测试。
- 安全：通过。无新增外部通信；预览与提交沿用 Host/Origin/JSON、XML 体积/递归和引用校验。界面文本渲染，选择不执行脚本，取消不写用户工作区。
- 验证：`go test ./...`、`go test -race ./internal/...`、`go vet ./...` 通过；前端 3 个单元测试、TypeScript/Vite 构建通过。Playwright 6 个完整场景通过（48.4 秒），最终错误提示修正后选择导入场景再次通过（7.4 秒）。覆盖 181 类型中仅选 A/D、完整字典、仅补充 Heartbeat、单接口新服务、取消、无效 XML、导入失败重试及刷新恢复；全量模板测试现在显式全选。
- 交付：`wails build -s -skipbindings -o TcpConnect-updated.exe` 成功。当前原程序仍在运行，保留用户进程和数据，交付 `build/bin/TcpConnect-updated.exe`。既有 Monaco 包体提示保持。
- 结论：通过，无未解决 finding。新增流程通过隔离的真实 HTTP/浏览器工作区验证；原生窗口未再次操作实际导入。
