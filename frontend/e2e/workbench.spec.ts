import { test,expect } from '@playwright/test'
import net from 'node:net'
import { once } from 'node:events'
import { fileURLToPath } from 'node:url'

async function setPort(page:any,port:number){await page.getByRole('button',{name:'服务设置'}).click();const modal=page.getByRole('dialog');await modal.locator('.n-input-number input').first().fill(String(port));await modal.getByRole('button',{name:'保存服务',exact:true}).click();await expect(modal).not.toBeVisible()}
async function editScript(page:any,tab:string,script:string){await page.getByRole('button',{name:tab,exact:true}).click();const input=page.locator('.code-editor textarea').first();await input.focus();await input.press('ControlOrMeta+A');await page.keyboard.insertText(script)}

test('TCP current connection, scripts, timers and persistence',async({page})=>{
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
  const peers=new Set<net.Socket>();const server=net.createServer(c=>{peers.add(c);c.on('data',b=>c.write(b));c.on('close',()=>peers.delete(c))});server.listen(0,'127.0.0.1');await once(server,'listening');const port=(server.address() as net.AddressInfo).port
  try{
    await page.goto('/');await page.getByRole('complementary').getByRole('button',{name:'TCP Echo 回显',exact:true}).click();await setPort(page,port);await page.getByRole('complementary').getByRole('button',{name:'TCP Echo 回显',exact:true}).click()
    await page.getByRole('button',{name:'连接',exact:true}).click();await expect(page.locator('.status-pill')).toHaveText('已连接')
    await editScript(page,'前置脚本',`return data.replace("Hello", "Verified");`)
    await editScript(page,'后置脚本',`console.log("response",data); if(!vars.get("started")){ vars.set("started",true); timers.setInterval("test-pulse",100,()=>socket.send("pulse"+String.fromCharCode(10))); }`)
    await page.getByRole('button',{name:'发送报文',exact:true}).click();await expect(page.locator('.message-entry').filter({hasText:'Verified, TCP!'}).first()).toBeVisible();await expect.poll(()=>peers.size).toBe(1)
    await page.getByRole('button',{name:/定时任务/}).click();await expect(page.locator('.task-row')).toContainText('test-pulse');await page.getByRole('button',{name:'停止任务',exact:true}).click();await expect(page.locator('.task-row')).toHaveCount(0)
    await page.getByRole('button',{name:/脚本与事件/}).click();await expect(page.locator('.console-list')).toContainText('response')
    await page.getByRole('button',{name:'断开',exact:true}).click();await expect(page.locator('.status-pill')).toHaveText('未连接');await page.reload();await page.getByRole('complementary').getByRole('button',{name:'TCP Echo 回显',exact:true}).click();await page.getByRole('button',{name:'前置脚本',exact:true}).click();await expect(page.locator('.code-editor')).toContainText('Verified')
    expect(errors).toEqual([])
  }finally{for(const c of peers)c.destroy();server.close()}
})

function fix(type:string,seq:number,body=''){const soh='\x01';const iso=new Date().toISOString();const ts=iso.slice(0,10).replace(/-/g,'')+'-'+iso.slice(11,23);const content=`35=${type}${soh}49=Fix${soh}56=FixClient${soh}34=${seq}${soh}52=${ts}${soh}${body}`;const raw=`8=FIX.4.4${soh}9=${Buffer.byteLength(content)}${soh}${content}`;const sum=Buffer.from(raw).reduce((a,b)=>(a+b)%256,0);return raw+`10=${String(sum).padStart(3,'0')}${soh}`}
test('FIX forms from XML, logon, TestRequest and dictionary import',async({page})=>{
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));const peers=new Set<net.Socket>();const received:string[]=[]
  const server=net.createServer(c=>{peers.add(c);let buffer='';c.on('close',()=>peers.delete(c));c.on('data',b=>{buffer+=b.toString();let match;while((match=/\x0110=\d{3}\x01/.exec(buffer))){const end=match.index+8;const raw=buffer.slice(0,end);buffer=buffer.slice(end);received.push(raw);if(raw.includes('\x0135=A\x01')){c.write(fix('A',1,'98=0\x01108=30\x01141=Y\x01'));setTimeout(()=>c.write(fix('1',2,'112=e2e-probe\x01')),100)}}})});server.listen(0,'127.0.0.1');await once(server,'listening');const port=(server.address() as net.AddressInfo).port
  try{
    await page.goto('/');await page.getByRole('complementary').getByRole('button',{name:'FIX 登录 Logon',exact:true}).click();await setPort(page,port);await page.getByRole('complementary').getByRole('button',{name:'FIX 登录 Logon',exact:true}).click();await expect(page.locator('.fix-form-scroll')).toContainText('EncryptMethod');await expect(page.locator('.fix-form-scroll')).toContainText('Username')
    await page.getByRole('button',{name:'连接',exact:true}).click();await page.getByRole('button',{name:'发送报文',exact:true}).click();await expect(page.locator('.status-pill')).toHaveText('FIX 已登录');await expect.poll(()=>received.some(r=>r.includes('112=e2e-probe'))).toBeTruthy();expect(peers.size).toBe(1)
    await page.getByRole('button',{name:'FIX 字段',exact:true}).click();await expect(page.locator('.decoded-table')).toContainText('TestReqID');await page.screenshot({path:'../.local/screenshots/fix-workbench.png',fullPage:true})
    await page.getByRole('complementary').getByRole('button',{name:'FIX 新建订单',exact:true}).click();await expect(page.locator('.fix-form-scroll')).toContainText('Account')
    const field=(name:string)=>page.locator('.fix-field-row').filter({has:page.locator('label',{hasText:new RegExp('^\\*?'+name+'\\d')})})
    await field('ClOrdID').locator('input').fill('E2E-ORDER')
    await field('Account').locator('input').fill('TEST-ACCOUNT')
    await field('Symbol').locator('input').fill('TEST')
    await field('OrderQty').locator('input').fill('100')
    for(const [name,option] of [['Side','1 · BUY'],['OrdType','1 · MARKET']]){await field(name).locator('.n-base-selection').click();await page.getByText(option,{exact:true}).click()}
    await field('TransactTime').getByRole('button',{name:'现在'}).click()
    await page.getByRole('checkbox',{name:'显示可选字段'}).check();await expect(page.locator('.fix-form-scroll')).toContainText('OutsideRth')
    const group=page.locator('.group-box').filter({has:page.locator('.component-title > span',{hasText:/^NoAllocs /})})
    await group.getByRole('button',{name:'＋ 添加记录',exact:true}).first().click()
    await field('AllocAccount').locator('input').fill('ALLOC-1')
    await page.getByRole('button',{name:'发送报文',exact:true}).click()
    await expect.poll(()=>received.some(r=>r.includes('35=D\x01')&&r.includes('78=1\x0179=ALLOC-1\x01')&&r.includes('11=E2E-ORDER\x01'))).toBeTruthy()
    await page.screenshot({path:'../.local/screenshots/fix-order-form.png',fullPage:true})
    await page.getByRole('button',{name:'断开',exact:true}).click()
    await page.getByTitle('协议字典',{exact:true}).click();await expect(page.locator('.dictionary-card').first()).toContainText('181');await page.locator('input[type=file][accept=".xml"]').setInputFiles(fileURLToPath(new URL('../../assets/fixspec/FIX44.xml',import.meta.url)))
    const imported=page.locator('.tree-service').filter({has:page.locator('.service-tree-row > span',{hasText:/^FIX44$/})})
    await expect(imported.locator('.tree-request-row')).toHaveCount(181)
    await imported.getByRole('button',{name:'FIX NewOrderSingle (D)',exact:true}).click();await expect(page.locator('.fix-form-scroll')).toContainText('Account')
    await page.getByTitle('协议字典',{exact:true}).click();await expect(page.locator('.dictionary-card')).toHaveCount(2)
    await page.screenshot({path:'../.local/screenshots/protocol-library.png',fullPage:true});expect(errors).toEqual([])
  }finally{for(const c of peers)c.destroy();server.close()}
})

test('import builds message templates and tree actions persist safely',async({page})=>{
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
  const xml=`<fix type="FIX" major="4" minor="4"><messages><message name="CustomProbe" msgtype="UQ" msgcat="app"><component name="Payload" required="Y"/></message><message name="CustomReply" msgtype="UR" msgcat="app"><field name="Result" required="Y"/></message></messages><components><component name="Payload"><field name="ProbeID" required="Y"/><field name="State" required="N"/></component></components><fields><field number="9001" name="ProbeID" type="STRING"/><field number="9002" name="State" type="CHAR"><value enum="Y" description="READY"/></field><field number="9003" name="Result" type="STRING"/></fields></fix>`
  const tree=(name:string)=>page.locator('.tree-service').filter({has:page.locator('.service-tree-row > span',{hasText:name})})
  await page.goto('/');await page.getByTitle('协议字典',{exact:true}).click()
  await page.locator('input[type=file][accept=".xml"]').setInputFiles({name:'CustomProtocol.xml',mimeType:'text/xml',buffer:Buffer.from(xml)})
  await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(2)
  await expect(page.getByRole('textbox',{name:'请求名称',exact:true})).toHaveValue('CustomProbe (UQ)')
  await expect(page.locator('.fix-form-scroll')).toContainText('ProbeID')
  await page.locator('.fix-field-row').filter({hasText:'ProbeID'}).locator('input').fill('keep-my-value')
  await page.getByRole('button',{name:'保存',exact:true}).click()
  await page.getByTitle('补全消息模板 CustomProtocol',{exact:true}).click()
  await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(2)
  await expect(page.locator('.fix-field-row').filter({hasText:'ProbeID'}).locator('input')).toHaveValue('keep-my-value')
  await page.screenshot({path:'../.local/screenshots/imported-message-templates.png',fullPage:true})
  // A failed save must leave the record visible and persisted.
  await page.route('**/api',async route=>{if(route.request().postDataJSON()?.method==='Save')await route.fulfill({json:{error:'Simulated storage failure'}});else await route.continue()})
  await page.getByTitle('删除消息 CustomProbe (UQ)',{exact:true}).click();await page.getByRole('button',{name:'删除',exact:true}).click()
  await expect(page.locator('.toast.error')).toContainText('Simulated storage failure')
  await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(2)
  await page.unroute('**/api')
  await tree('CustomProtocol').getByRole('button',{name:'FIX CustomReply (UR)',exact:true}).click()
  await page.getByTitle('删除消息 CustomProbe (UQ)',{exact:true}).click();await page.getByRole('button',{name:'删除',exact:true}).click()
  await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(1)
  await expect(page.getByRole('textbox',{name:'请求名称',exact:true})).toHaveValue('CustomReply (UR)')
  await page.getByTitle('补全消息模板 CustomProtocol',{exact:true}).click();await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(2)
  await tree('CustomProtocol').getByTitle('新增请求',{exact:true}).click()
  const modal=page.getByRole('dialog');await modal.locator('.n-base-selection').click();await page.locator('.n-base-select-option__content').getByText('CustomReply (UR)',{exact:true}).click();await modal.getByRole('button',{name:'创建消息',exact:true}).click()
  await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(3)
  await page.getByTitle('删除消息 CustomReply (UR)',{exact:true}).last().click();await page.getByRole('button',{name:'删除',exact:true}).click()
  await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(2)
  // Creating a service from an existing dictionary also builds every template.
  await page.getByTitle('协议字典',{exact:true}).click()
  await page.locator('.dictionary-card').filter({hasText:'CustomProtocol.xml'}).getByRole('button',{name:'创建 FIX 服务',exact:true}).click()
  await page.getByRole('dialog').getByRole('textbox').first().fill('Second custom service')
  await page.getByRole('dialog').getByRole('button',{name:'保存服务',exact:true}).click()
  await expect(tree('Second custom service').locator('.tree-request-row')).toHaveCount(2)
  await tree('CustomProtocol').getByRole('button',{name:'FIX CustomReply (UR)',exact:true}).click()
  await page.getByTitle('删除服务 Second custom service',{exact:true}).click();await page.getByRole('button',{name:'删除',exact:true}).click()
  await expect(tree('Second custom service')).toHaveCount(0)
  await expect(page.getByRole('textbox',{name:'请求名称',exact:true})).toHaveValue('CustomReply (UR)')
  await page.reload();await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(2)
  await page.getByTitle('删除服务 CustomProtocol',{exact:true}).click();await page.getByRole('button',{name:'删除',exact:true}).click()
  await expect(tree('CustomProtocol')).toHaveCount(0)
  await page.reload();await expect(tree('CustomProtocol')).toHaveCount(0)
  expect(errors).toEqual([])
})

test('service tree search filters messages, expands matches and restores all records',async({page})=>{
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
  await page.goto('/')
  await page.locator('input[type=file][accept=".xml"]').setInputFiles({name:'SearchProtocol.xml',mimeType:'text/xml',buffer:await (await import('node:fs/promises')).readFile(fileURLToPath(new URL('../../assets/fixspec/FIX44.xml',import.meta.url)))})
  const tree=page.locator('.tree-service').filter({has:page.locator('.service-tree-row > span',{hasText:/^SearchProtocol$/})})
  await expect(tree.locator('.tree-request-row')).toHaveCount(181)
  const originalName=await page.getByRole('textbox',{name:'请求名称',exact:true}).inputValue()
  // A collapsed service must open when a matching message is searched.
  await tree.locator('.service-tree-row').click();await expect(tree.locator('.tree-request-row')).toHaveCount(0)
  const search=page.getByPlaceholder('搜索服务或请求',{exact:true})
  await search.fill('  lOgOn  ')
  await expect(tree.locator('.tree-request-row')).toHaveCount(1)
  await expect(tree.locator('.tree-request-button')).toHaveText('FIXLogon (A)')
  await expect(page.locator('.tree-request-button').filter({hasText:'Heartbeat'})).toHaveCount(0)
  await expect(page.locator('.tree-service').filter({has:page.locator('.service-tree-row > span',{hasText:/^本地 TCP$/})})).toHaveCount(0)
  await tree.locator('.tree-request-button').click()
  await expect(page.getByRole('textbox',{name:'请求名称',exact:true})).toHaveValue(originalName)
  await page.screenshot({path:'../.local/screenshots/message-search.png',fullPage:true})
  // Searching a service name shows its messages; a message type can also be searched.
  await search.fill('searchprotocol');await expect(tree.locator('.tree-request-row')).toHaveCount(181)
  await search.fill('1002re');await expect(tree.locator('.tree-request-row')).toHaveCount(1)
  await expect(tree.locator('.tree-request-button')).toContainText('LoginAccount (1002RE)')
  await search.fill('no-such-message-12345');await expect(page.locator('.tree-service')).toHaveCount(0)
  await expect(page.locator('.empty-tree')).toContainText('没有匹配的服务或消息')
  await expect(page.getByRole('textbox',{name:'请求名称',exact:true})).toHaveValue(originalName)
  await page.getByRole('button',{name:'清除搜索',exact:true}).click();await expect(search).toHaveValue('')
  await expect(tree.locator('.tree-request-row')).toHaveCount(181)
  await search.fill('   ');await expect(tree.locator('.tree-request-row')).toHaveCount(181)
  // Filtering must never remove saved records, including during an explicit save.
  await search.fill('Logon');await page.getByRole('button',{name:'保存',exact:true}).click();await page.reload()
  await expect(tree.locator('.tree-request-row')).toHaveCount(181)
  expect(errors).toEqual([])
})

test('TCP receive rule binds a custom function and frames split and joined data',async({page})=>{
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
  const peers=new Set<net.Socket>()
  const server=net.createServer(c=>{peers.add(c);let started=false;c.on('close',()=>peers.delete(c));c.on('data',()=>{if(started){c.write('!');return};started=true;c.write('one!tw');setTimeout(()=>{if(!c.destroyed)c.write('o!tail')},100)})})
  server.listen(0,'127.0.0.1');await once(server,'listening');const port=(server.address() as net.AddressInfo).port
  const functionName='按感叹号接收完整消息'
  try{
    await page.goto('/');await page.locator('.sidebar-search').getByTitle('新建',{exact:true}).click()
    await page.getByText('TCP（Socket）服务',{exact:true}).click()
    const modal=page.getByRole('dialog')
    await modal.getByRole('textbox').first().fill('Custom framing TCP')
    await modal.locator('.n-input-number input').first().fill(String(port))
    await modal.getByRole('button',{name:'管理接收判断函数',exact:true}).click()
    await expect(page.locator('.function-kind-tabs button').filter({hasText:'接收完成判断'})).toHaveClass(/active/)
    await page.getByRole('button',{name:'新建函数',exact:true}).first().click()
    await page.locator('.function-editor-header input').fill(functionName)
    const editor=page.locator('.code-editor textarea').first();await editor.focus();await editor.press('ControlOrMeta+A')
    await page.keyboard.insertText('const end = bytes.indexOf(33); return end < 0 ? 0 : end + 1;')
    await page.getByRole('button',{name:'绑定并返回服务设置',exact:true}).click()
    await expect(modal.getByRole('textbox').first()).toHaveValue('Custom framing TCP')
    await expect(modal.locator('.n-input-number input').first()).toHaveValue(String(port))
    await expect(modal.locator('.receive-rule-select')).toContainText(functionName)
    await expect(modal).toContainText('已绑定：'+functionName)
    // Built-in rules and custom functions share the same selector.
    await modal.locator('.receive-rule-select').click();await page.getByText('包含分隔符',{exact:true}).click()
    await modal.locator('.receive-rule-select').click()
    await expect(page.getByText('自定义接收完成判断函数',{exact:true})).toBeVisible()
    await expect(page.locator('.n-base-select-option').filter({hasText:'竖线转换为 SOH'})).toHaveCount(0)
    await page.locator('.n-base-select-option__content').getByText(functionName,{exact:true}).click()
    await page.screenshot({path:'../.local/screenshots/custom-receive-rule.png',fullPage:true})
    await modal.getByRole('button',{name:'保存服务',exact:true}).click();await expect(modal).not.toBeVisible()
    await page.getByRole('button',{name:'保存',exact:true}).click();await page.reload()
    const service=page.locator('.tree-service').filter({has:page.locator('.service-tree-row > span',{hasText:'Custom framing TCP'})})
    await service.locator('.tree-request-button').click();await page.getByRole('button',{name:'服务设置',exact:true}).click()
    await expect(modal.locator('.receive-rule-select')).toContainText(functionName)
    await modal.getByRole('button',{name:'编辑或更换绑定函数',exact:true}).click()
    await expect(page.locator('.function-editor-header input')).toHaveValue(functionName)
    await page.getByRole('button',{name:'返回服务设置',exact:true}).click()
    await modal.getByRole('button',{name:'取消',exact:true}).click()
    const input=page.locator('.code-editor textarea').first();await input.focus();await input.press('ControlOrMeta+A');await page.keyboard.insertText('start')
    await page.getByRole('button',{name:'连接',exact:true}).click();await page.getByRole('button',{name:'发送报文',exact:true}).click()
    const received=page.locator('.message-entry').filter({has:page.locator('.receive-icon')})
    await expect(received).toHaveCount(2);await expect(received.locator('code')).toHaveText(['two!','one!'])
    await page.getByRole('button',{name:'发送报文',exact:true}).click()
    await expect(received).toHaveCount(3);await expect(received.first().locator('code')).toHaveText('tail!')
    expect(peers.size).toBe(1);await page.getByRole('button',{name:'断开',exact:true}).click();expect(errors).toEqual([])
  }finally{for(const c of peers)c.destroy();server.close()}
})
