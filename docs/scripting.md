# JavaScript 脚本契约

## 执行位置

前置脚本：用户点击发送后执行；`data` 为编码后 UTF-8 文本，`bytes` 为原始字节数组。返回字符串或 Uint8Array 替换待发数据，不返回则保留原数据。

后置脚本：每条完整接收报文执行；data/bytes 始终对应原始接收数据，返回值不改变展示，展示转换请配置返回数据处理函数。

请求/返回处理函数：必须返回字符串或 Uint8Array/字节数组，不返回视为错误。FIX 托管模式最后自动维护头尾，脚本无需计算 9/10。

接收判断函数：参数同上，返回完整首帧字节数；0 表示未完成。框架消费一帧后使用剩余字节再次调用；不要返回剩余缓冲区总长度。兼容 boolean 返回，但 true 会把全部缓冲作为一帧，不能拆粘包。输入 bytes 比 data.length 更适合长度计算。

```javascript
// 换行分帧：包含换行在内
const i = bytes.indexOf(10);
return i < 0 ? 0 : i + 1;
```

## 当前连接 API

```javascript
socket.send("ping\n");
socket.send(new Uint8Array([0, 1, 255]));
socket.isConnected();
socket.disconnect();

console.log("received", data);
console.info("info");
console.warn("warning");
console.error("error");

vars.set("key", "value");
vars.get("key");
vars.has("key");
vars.unset("key");
// 兼容别名，均为当前连接的会话变量
pm.environment.set("key", "value");
pm.variables.get("key");
pm.globals.get("key");
```

`socket.send()` 通过当前请求关联的请求处理函数发送；不会重新执行前置和后置脚本。FIX 托管请求上下文中会自动维护客户端序号及头尾。rawMode 上下文中按原样发送。脚本环境无 apt、fetch、require、pm.sendRequest。

## 定时任务

```javascript
// 后置脚本中：启动一次，定时使用当前连接发送
if (!vars.get("heartbeatStarted")) {
  vars.set("heartbeatStarted", true);
  timers.setInterval("heartbeat", 30000, () => {
    socket.send("ping\n");
    console.log("sent", new Date().toISOString());
  });
}
```

`timers.setInterval(name, milliseconds, callback)` 循环执行；`timers.setTimeout` 执行一次；`timers.clear(name)` 停止。同名任务替换，至少 50ms，最多 32 个。全局 setInterval/setTimeout/clearInterval/clearTimeout 支持标准参数形式，返回可清理的任务名称。回调捕获创建时请求的处理配置，避免切换请求导致定时器使用错误函数。

FIX 会话内置心跳，不需要再用脚本发送第二套心跳。脚本定时器可用于周期业务请求：

```javascript
timers.setInterval("probe", 30000, () => {
  // 35=1 是 TestRequest，112 为请求标识；管理模式自动添加其他头尾
  socket.send("35=1\x01112=probe-" + Date.now() + "\x01");
});
```

每次收到响应都会运行后置脚本，所以重复创建的任务应使用会话变量保护，否则同名任务会不断重置。手动停止任务后如需重新启动，可 `vars.unset("heartbeatStarted")`，或断开重新连接。

## 编码工具

```javascript
const SOH = String.fromCharCode(1);
const encoded = new TextEncoder().encode("文本");
new TextDecoder().decode([65,66]);
hex.encode(new Uint8Array([0,255])); // "00ff"
hex.decode("00 ff"); // 字节数组
base64.encode("hello");
base64.decode("aGVsbG8=");
```

脚本以函数体执行，已注入 `data`、`bytes` 参数，不要再次用 const 声明这两个名称。推荐 `const encoded = new TextEncoder().encode(data)`。

## 前置与返回处理示例

接收完成判断函数在“函数 → 接收完成判断”中创建，在 TCP 服务的“接收完成规则”下拉框中直接选择函数名。服务设置提供管理入口；新建/编辑后可“绑定并返回服务设置”，保存服务并重新连接生效。此处仅列出接收完成判断类函数，请求和返回处理函数仍在请求的“处理设置”中绑定。

```javascript
// 包含第一个换行符的字节数；没有完整消息则等待更多数据。
const end = bytes.indexOf(10);
return end < 0 ? 0 : end + 1;
```

返回字节数后，运行时取出该帧，并对剩余缓冲继续调用函数，因此可同时处理分包与粘包。

```javascript
// 前置脚本：动态替换模板
return data.replace("{{timestamp}}", new Date().toISOString());
```

```javascript
// 请求处理：展示分隔符转 SOH
return data.replace(/\|/g, "\x01");
```

```javascript
// 返回处理：只改变展示，不改变原始数据
return data.replace(/\x01/g, "|");
```

所有错误在“脚本与事件”显示，1500ms 超时会被中断；周期任务异常后停止。提供的是本地脚本环境，不承诺执行不可信代码的内存隔离。
