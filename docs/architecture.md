# 架构

## 入口和依赖

`main.go` 装配 SQLite、SessionFactory、Workbench 和 Bridge。Wails 原生绑定与本机 HTTP RPC 均调用相同 Bridge。Vue 通过 `frontend/src/services/api.ts` 选择桥接方式。

```text
Vue 工作台 / FIX 动态表单
           ↓
interfaces.Bridge（Wails / loopback HTTP）
           ↓
application.Workbench（配置、连接、发送、导入导出、事件）
           ↓
domain（Service / Request / Function / Repository / Session）
           ↑ 实现端口
infrastructure（runtime / storage）
```

`domain/fix` 是独立协议域，处理 QuickFIX 字典、字段规则、编码和分帧；不引用应用、UI、网络或 SQL。runtime 的通用 TCP 与脚本由 `session.go`、`scripts.go` 实现，FIX 会话策略集中在 `fix_session.go`。普通 TCP 会话不加载字典。

## 并发与生命周期

连接持续读取到一个有界缓冲。运行时互斥区串行执行用户发送、接收处理、脚本、脚本定时回调、FIX 会话发送。Goja 只在该互斥区内访问。socket.send 是当前脚本内部的同步原生调用，不重新进入前后置生命周期。写操作设置超时并处理部分写入。

命名定时器绑定创建时的请求处理上下文；同名替换，数量最多 32。断开关闭 Socket 并取消全部脚本任务；FIX 心跳循环观察连接状态退出。脚本最大执行 1500ms，异常记录到事件流。定时任务异常会停止该任务。

接收：原始字节 → 拆分完整帧（保留尾部残包）→ 返回处理函数 → 展示事件；FIX 模块消费协议字段；后置脚本在每条完整响应上执行。返回处理不改原始字节。

发送：模板/字典表单 → 前置脚本 → 请求处理函数 → FIX 托管头尾（仅 FIX 非原始模式）→ 串行写入 → 发送记录。定时 socket.send 从请求处理函数开始，不重复运行前后置脚本。

## 存储

SQLite 的单行 workspace 存储版本化 JSON，参数化原子 UPSERT。工作区包含聚合配置及 ID 引用，保存前校验引用和字典。连接、JS VM、定时器、收发历史为瞬态资源。默认 `%APPDATA%/TcpConnect`，`--data-dir` 可覆盖。导出包含配置，导入替换配置。

事件最多 500 条且前后端均按载荷总量限制；FIX 重发历史最多 1000 条/16 MiB，序号缺口缓冲最多 100 条/16 MiB；接收缓冲和单报文最多 4 MiB；XML 最大 8 MiB，展开深度最大 32、节点数最大 200000。原始二进制通过 HEX/Base64 精确保留，文本视图按 UTF-8 展示。取消连接会使进行中的连接尝试失效，迟到的 Socket 立即关闭；删除服务和导入工作区前检查活动连接。

## XML 与界面

导入解析保留 XML 子节点顺序，解析 component 引用并检测循环，group 保留嵌套和计数字段。Vue FixFields 递归渲染，组件展开和重复组记录仍使用字典顺序；多值枚举支持多选。数量与价格作为字符串编辑，避免 JS 浮点数损失精度。可选组件未启用时不触发子字段必填，启用后即按字典校验；重复组每条记录必须填写分隔首字段。

`application/protocols.go` 编排 XML 导入、服务和消息模板创建。`ImportProtocol` 在一个工作区保存中创建字典、同名服务及所有消息；`SaveService` 为新增 FIX 服务生成全部模板；`GenerateMessages` 按消息类型补全缺失用例，不覆盖现有内容；`CreateMessage` 从指定字典类型创建命名用例。候选工作区经过数量/引用校验及持久化成功后才替换内存状态，失败不留半成品。桌面与 HTTP Bridge 暴露相同用例，领域解析保持独立。

服务与消息删除由界面构造候选工作区，保存成功后更新服务树与标签。删除服务前先断开连接，使运行时清理任务；保存失败保留记录，但已断开的连接需手动重连。消息切换清空旧预览，并忽略旧请求迟到的预览响应。

TCP 接收规则在界面按内置规则和 frame 类函数分组；选择函数映射为 `framing=custom` 与 `frameFunctionId`，沿用已有应用校验及运行时分帧执行。切换内置规则清除函数关联，服务草稿在进入函数管理时保留，显式绑定后回到服务设置。配置在重新连接时加载，不改变活动连接的分帧器。

## 构建

无 proto/代码生成链路。`npm --prefix frontend run build` 生成前端静态资源，Go embed 打包；`wails build -skipbindings` 生成 Windows EXE。桥接 API 动态调用，无需前端依赖生成的 Wails JS 文件。`go test` 前需要存在 frontend/dist（执行前端构建）。
