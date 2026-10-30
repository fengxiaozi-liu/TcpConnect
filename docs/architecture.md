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

接收：原始字节 → 拆分完整帧（保留尾部残包）→ 返回处理函数 → 展示事件；FIX 模块消费协议字段；后置脚本在每条已匹配到请求的完整响应上执行。返回处理不改原始字节。

发送：模板/字典表单 → 前置脚本 → 请求处理函数 → FIX 托管头尾（仅 FIX 非原始模式）→ 串行写入 → 发送记录。定时 socket.send 从请求处理函数开始，不重复运行前后置脚本。

## 存储

SQLite 的单行 workspace 存储版本化 JSON，参数化原子 UPSERT。工作区包含聚合配置及 ID 引用，保存前校验引用和字典。连接、JS VM、定时器、收发历史为瞬态资源。默认 `%APPDATA%/TcpConnect`，`--data-dir` 可覆盖。导出包含配置，导入替换配置。

事件最多 500 条且前后端均按载荷总量限制；FIX 重发历史最多 1000 条/16 MiB，序号缺口缓冲最多 100 条/16 MiB；接收缓冲和单报文最多 4 MiB；XML 最大 8 MiB，展开深度最大 32、节点数最大 200000。原始二进制通过 HEX/Base64 精确保留，文本视图按 UTF-8 展示。取消连接会使进行中的连接尝试失效，迟到的 Socket 立即关闭；删除服务和导入工作区前检查活动连接。

## XML 与界面

导入解析保留 XML 子节点顺序，解析 component 引用并检测循环，group 保留嵌套和计数字段。Vue FixFields 递归渲染，每个作用域过滤后稳定排序，将必填项置前；不原地修改定义，编码和组分隔首字段仍遵循字典顺序。多值枚举支持多选。数量与价格作为字符串编辑，避免 JS 浮点数损失精度。可选组件未启用时不触发子字段必填，启用后即按字典校验；重复组每条记录必须填写分隔首字段。

`application/protocols.go` 编排 XML 导入、服务和消息模板创建。`PreviewDictionary` 只解析领域字典，不持久化；前端共用 `MessagePicker.vue` 提供按名称/类型搜索、多选、结果全选和清空。`ImportProtocol` 接收显式消息类型列表，在一个工作区保存中创建完整字典、同名服务及选中模板；`SaveService` 可以单独保存服务（模板可选）；`GenerateMessages` 只补充勾选的缺失类型，不覆盖现有内容；`CreateMessage` 从指定字典类型创建命名用例。ImportProtocol/GenerateMessages 的空选择和无效类型直接拒绝，SaveService 的空选择表示只保存服务；重复类型去重。候选工作区经过数量/引用校验及持久化成功后才替换内存状态，失败不留半成品。桌面与 HTTP Bridge 暴露相同用例，领域解析保持独立。

服务与消息删除由界面构造候选工作区，保存成功后更新服务树与标签。删除服务前先断开连接，使运行时清理任务；保存失败保留记录，但已断开的连接需手动重连。消息切换清空旧预览，并忽略旧请求迟到的预览响应。

TCP 接收规则在界面按内置规则和 frame 类函数分组；选择函数映射为 `framing=custom` 与 `frameFunctionId`，沿用已有应用校验及运行时分帧执行。切换内置规则清除函数关联，服务草稿在进入函数管理时保留，显式绑定后回到服务设置。配置在重新连接时加载，不改变活动连接的分帧器。

## 构建

无 proto/代码生成链路。`npm --prefix frontend run build` 生成前端静态资源，Go embed 打包；`wails build -skipbindings` 生成 Windows EXE。桥接 API 动态调用，无需前端依赖生成的 Wails JS 文件。`go test` 前需要存在 frontend/dist（执行前端构建）。

## FIX 多版本

`domain/fix` 统一识别 QuickFIX FIX.4.0～FIX.4.4，旧字典省略 type 时按 FIX 处理；5.x/FIXT 明确拒绝。Dictionary.Version 来自 XML 根节点，Encode 要求调用方显式传入版本，预览、托管发送、会话管理消息、重发与 GapFill 均使用绑定字典的版本。FrameLength 对支持版本增量匹配头部，继续以字节长度及校验和验证边界；会话消费前再检查 BeginString 与绑定版本一致，避免版本混用改变登录/序号状态。

自动登录使用 Dictionary.Body 序列化本版本 Logon，忽略未声明的可选凭据/重置字段，缺少业务必填字段时提示手动登录。4.0/4.1 发送 UTC 秒精度；4.2 起使用毫秒。Body 校验兼容旧 CHAR 的字符串语义及 TIME/DATE、UTCDATE、QUANTITY；Vue 递归表单的时间按钮继承当前版本。多个服务可以各自绑定不同字典，TCP 运行时不增加协议版本配置。连接采用建立时的字典快照，更换字典需重新连接。

## 手选协议与 XML 扩展

Service.fixVersion 保存手动选择的 FIX.4.0～FIX.4.4，dictionaryId 可以为空。标准 XML 通过 assets/fixspec/embed.go 编译进程序，领域 ResolveDictionary 根据版本取内置标准，或从用户 XML 根节点推导版本；存在显式版本与 XML 不一致时拒绝。业务 FIX44.xml 保持独立，仅自定义导入/旧示例继续使用。

应用 serviceDictionary 统一解析服务的标准/自定义定义，供保存校验、CreateMessage、GenerateMessages 和 Bridge.Preview 使用；runtime Factory.Connect 使用相同领域解析入口。界面通过 BuiltinDictionary 获取标准元数据，Wails 与 HTTP 同步暴露。标准服务不向用户协议库插入虚拟字典；旧工作区的 dictionaryId 绑定仍可解析，缺少 fixVersion 时使用 XML 根节点。选择窗口按标准版本或自定义字典缓存，保留 TCP 独立性。

新建 FIX 保存直接提交，之后可单独添加消息；不再强制先选模板，不调用 XML 导入。新建/编辑以协议版本选项展示内置标准及已有自定义扩展，选择内置版本即解除自定义引用。导入 XML 仍先预览、自动识别版本、勾选接口后原子创建，文件名仅作名称。构建标识位于页脚，用有日期的 exe 交付避免打开旧包。

## 请求关联与局部协议定义

`domain.Request` 可保存description、customNodes和messageMode；customNodes缺省使用服务标准/XML节点，显式空数组表示无业务字段。`domain/fix/custom.go` 校验局部节点，禁止重复/保留Tag、重复名称和非法组结构；Body与TextBody用于表单及管理文本，Bridge预览与runtime发送通过Request.Body共用同一规则。文本检查必填、类型、枚举及组计数后保留原有业务字段顺序，保留可选头字段；自动头尾仍由Encode负责。保存校验每个服务只解析一份定义，多个自定义请求复用，避免逐请求重复解析XML。

`runtime/correlation.go` 在成功写入后记录不可变的请求快照和实际报文字段。连接互斥锁保证响应读处理不会早于登记。普通TCP消费FIFO；FIX按业务标识、引用序号或手动登录/退出匹配，学习OrderID供后续更新使用，重置Logon清空旧匹配。最多256个发送上下文、请求JSON快照总计8MiB，单标识至多1024字节/每条最多64个；超出窗口的响应归入会话记录。自动管理消息不登记请求。收发及定时回调在临时上下文内执行并恢复，避免脚本、日志和socket.send误用另一请求。

`Event.requestId` 标识请求，空值表示会话。记录附加仅含实际Tag及对应消息/组节点的fixDefinition快照，避免字段编辑/协议切换后历史解码漂移；快照计入既有16MiB事件预算。原始字节、HEX/Base64不受解码影响。ClearRequestEvents按serviceId/requestId筛选，不重置事件游标；两种Bridge适配暴露相同入口。旧ClearEvents接口保持兼容，界面不再调用全局清空。

Vue的SchemaEditor递归维护请求局部结构，字段改名迁移相应作用域的值、删除清理值，组件共享父作用域，重复组作用于各条记录。FixFields采用单列字段表和可展开组。文档、结构编辑、表单和解码使用一致元数据；消息编辑器的文本发送与表单生成模式显式区分，切换请求不覆盖编辑草稿。

## 心跳状态与请求周期发送

SessionState的Heartbeat返回等待登录/运行/断开、实际协商间隔和最近成功发送35=0的时间，Tasks保留名称兼容，TaskDetails补充类型、请求和间隔。FIX内置循环仍以空闲时间触发，不计入脚本任务限额。

Bridge与Workbench的StartRepeat委托Session端口；runtime/repeat.go在连接锁内快照请求，使用request/<ID>名称，与脚本任务共享32个上限。ticker首次延迟一个间隔，再调用sendLocked，复用手动发送的校验、脚本、字典编码、串行写入及关联。失败/停止/断开清理，任务身份检查避免旧循环误取消同名新任务；重连不恢复任务。Goja仍只在连接互斥区内运行。

表单下仅单行预览，原始报文编辑器替换表单。Request.draftFromForm标记未经修改的生成稿，再次打开可从当前表单刷新；手工编辑、复制历史或开启跳过自动计算时清除此标记，保留草稿。messageMode=text默认仍托管头尾，rawMode独立控制异常测试。预览生成失败不切模式，异步返回只更新原请求对象。
