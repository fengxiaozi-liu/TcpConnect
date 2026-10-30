import { test,expect } from '@playwright/test'
import net from 'node:net'
import { once } from 'node:events'
import { fileURLToPath } from 'node:url'

async function setPort(page:any,port:number){await page.getByRole('button',{name:'服务设置'}).click();const modal=page.getByRole('dialog');await modal.locator('.n-input-number input').first().fill(String(port));await modal.getByRole('button',{name:'保存服务',exact:true}).click();await expect(modal).not.toBeVisible()}
async function editScript(page:any,tab:string,script:string){await page.getByRole('button',{name:tab,exact:true}).click();const input=page.locator('.code-editor textarea').first();await input.focus();await input.press('ControlOrMeta+A');await page.keyboard.insertText(script)}
async function chooseInterfaces(page:any,types?:string[]){const modal=page.locator('.message-picker');await expect(modal).toBeVisible();if(types){for(const name of types)await modal.getByRole('checkbox',{name,exact:true}).check()}else await modal.getByRole('button',{name:'全选当前结果',exact:true}).click();await modal.getByRole('button',{name:/^确认(导入|创建|添加)$/}).click();await expect(modal).not.toBeVisible()}


test('editor wheel scrolls the page for short content and at long content boundaries',async({page})=>{
  await page.setViewportSize({width:1440,height:800})
  await page.goto('/')
  await page.getByRole('complementary').getByRole('button',{name:'FIX 登录 Logon',exact:true}).click()
  await page.getByRole('checkbox',{name:'原始报文',exact:true}).click()
  const editor=page.locator('.request-editor-card .code-editor'),outer=page.locator('.workbench-scroll')
  await expect(editor).toContainText('35=A')
  const scroll=()=>outer.evaluate(el=>el.scrollTop)
  await outer.evaluate(el=>el.scrollTop=0)
  await editor.hover({position:{x:200,y:160}})
  await page.mouse.wheel(0,200)
  await expect.poll(scroll).toBeGreaterThan(0)
  await editor.hover({position:{x:200,y:40}})
  await page.mouse.wheel(0,-200)
  await expect.poll(scroll).toBe(0)

  const input=editor.locator('textarea')
  await input.focus();await input.press('ControlOrMeta+A')
  await page.keyboard.insertText(Array.from({length:100},(_,i)=>'58=scroll-'+i+'|').join('\n'))
  const inner=editor.locator('.monaco-scrollable-element.editor-scrollable')
  const viewTop=()=>editor.locator('.lines-content').evaluate(el=>parseFloat((el as HTMLElement).style.top)||0)
  await input.press('ControlOrMeta+Home')
  await outer.evaluate(el=>el.scrollTop=0)
  await editor.hover({position:{x:200,y:160}})
  await page.mouse.wheel(0,160)
  await expect.poll(viewTop).toBeLessThan(0)
  expect(await scroll()).toBe(0)
  await expect(inner).toBeVisible()
  await input.press('ControlOrMeta+End')
  await outer.evaluate(el=>el.scrollTop=0)
  await editor.hover({position:{x:200,y:160}})
  await page.mouse.wheel(0,160)
  await expect.poll(scroll).toBeGreaterThan(0)
  await input.press('ControlOrMeta+Home')
  await outer.evaluate(el=>el.scrollTop=150)
  await editor.hover({position:{x:200,y:160}})
  const previous=await scroll()
  await page.mouse.wheel(0,-120)
  await expect.poll(scroll).toBeLessThan(previous)
})

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

function fix(type:string,seq:number,body='',version='FIX.4.4'){const soh='\x01';const iso=new Date().toISOString();const ts=iso.slice(0,10).replace(/-/g,'')+'-'+iso.slice(11,23);const content=`35=${type}${soh}49=Fix${soh}56=FixClient${soh}34=${seq}${soh}52=${ts}${soh}${body}`;const raw=`8=${version}${soh}9=${Buffer.byteLength(content)}${soh}${content}`;const sum=Buffer.from(raw).reduce((a,b)=>(a+b)%256,0);return raw+`10=${String(sum).padStart(3,'0')}${soh}`}
test('FIX forms from XML, logon, TestRequest and dictionary import',async({page})=>{
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));const peers=new Set<net.Socket>();const received:string[]=[]
  const server=net.createServer(c=>{peers.add(c);let buffer='';c.on('close',()=>peers.delete(c));c.on('data',b=>{buffer+=b.toString();let match;while((match=/\x0110=\d{3}\x01/.exec(buffer))){const end=match.index+8;const raw=buffer.slice(0,end);buffer=buffer.slice(end);received.push(raw);if(raw.includes('\x0135=A\x01')){c.write(fix('A',1,'98=0\x01108=30\x01141=Y\x01'));setTimeout(()=>c.write(fix('1',2,'112=e2e-probe\x01')),100)}}})});server.listen(0,'127.0.0.1');await once(server,'listening');const port=(server.address() as net.AddressInfo).port
  try{
    await page.goto('/');await page.getByRole('complementary').getByRole('button',{name:'FIX 登录 Logon',exact:true}).click();await setPort(page,port);await page.getByRole('complementary').getByRole('button',{name:'FIX 登录 Logon',exact:true}).click();await expect(page.locator('.fix-form-scroll')).toContainText('EncryptMethod');await expect(page.locator('.fix-form-scroll')).toContainText('Username')
    await page.getByRole('button',{name:'连接',exact:true}).click();await page.getByRole('button',{name:'发送报文',exact:true}).click();await expect(page.locator('.status-pill')).toHaveText('FIX 已登录');await expect.poll(()=>received.some(r=>r.includes('112=e2e-probe'))).toBeTruthy();expect(peers.size).toBe(1)
    await page.getByRole('button',{name:'会话记录',exact:true}).click();await page.getByRole('button',{name:'FIX 字段',exact:true}).click();await expect(page.locator('.decoded-table')).toContainText('TestReqID');await page.screenshot({path:'../.local/screenshots/fix-workbench.png',fullPage:true})
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
    await page.getByTitle('协议字典',{exact:true}).click();await expect(page.locator('.dictionary-card').first()).toContainText('181');await page.locator('input[type=file][accept=".xml"]').setInputFiles(fileURLToPath(new URL('../../assets/fixspec/FIX44.xml',import.meta.url)));await chooseInterfaces(page)
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
  await page.locator('input[type=file][accept=".xml"]').setInputFiles({name:'CustomProtocol.xml',mimeType:'text/xml',buffer:Buffer.from(xml)});await chooseInterfaces(page)
  await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(2)
  await expect(page.getByRole('textbox',{name:'请求名称',exact:true})).toHaveValue('CustomProbe (UQ)')
  await expect(page.locator('.fix-form-scroll')).toContainText('ProbeID')
  await page.locator('.fix-field-row').filter({hasText:'ProbeID'}).locator('input').fill('keep-my-value')
  await page.getByRole('button',{name:'保存',exact:true}).click()
  await page.getByTitle('补全消息模板 CustomProtocol',{exact:true}).click();await expect(page.locator('.message-picker').getByRole('checkbox',{name:'CustomProbe (UQ)',exact:true})).toBeDisabled();await page.locator('.message-picker').getByRole('button',{name:'取消',exact:true}).click()
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
  await page.getByTitle('补全消息模板 CustomProtocol',{exact:true}).click();await chooseInterfaces(page);await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(2)
  await tree('CustomProtocol').getByTitle('新增请求',{exact:true}).click()
  const modal=page.getByRole('dialog');await modal.locator('.n-base-selection').click();await page.locator('.n-base-select-option__content').getByText('CustomReply (UR)',{exact:true}).click();await modal.getByRole('button',{name:'创建消息',exact:true}).click()
  await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(3)
  await page.getByTitle('删除消息 CustomReply (UR)',{exact:true}).last().click();await page.getByRole('button',{name:'删除',exact:true}).click()
  await expect(tree('CustomProtocol').locator('.tree-request-row')).toHaveCount(2)
  // Creating a service saves its configuration before separately adding templates.
  await page.getByTitle('协议字典',{exact:true}).click()
  await page.locator('.dictionary-card').filter({hasText:'CustomProtocol.xml'}).getByRole('button',{name:'创建 FIX 服务',exact:true}).click()
  await page.getByRole('dialog').getByRole('textbox').first().fill('Second custom service')
  await page.getByRole('dialog').getByRole('button',{name:'保存服务',exact:true}).click()
  await expect(page.getByRole('dialog')).not.toBeVisible();await expect(tree('Second custom service').locator('.tree-request-row')).toHaveCount(0)
  await page.getByRole('button',{name:'添加消息接口',exact:true}).click();await chooseInterfaces(page)
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
  await page.locator('input[type=file][accept=".xml"]').setInputFiles({name:'SearchProtocol.xml',mimeType:'text/xml',buffer:await (await import('node:fs/promises')).readFile(fileURLToPath(new URL('../../assets/fixspec/FIX44.xml',import.meta.url)))});await chooseInterfaces(page)
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

test('XML preview imports only selected interfaces and can cancel or retry safely',async({page})=>{
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
  const xml=await (await import('node:fs/promises')).readFile(fileURLToPath(new URL('../../assets/fixspec/FIX44.xml',import.meta.url)))
  const upload=()=>page.locator('input[type=file][accept=".xml"]').setInputFiles({name:'SelectiveProtocol.xml',mimeType:'text/xml',buffer:xml})
  const snapshot=async()=>{const r=await page.request.post('/api',{data:{method:'Snapshot',args:[0]}});return (await r.json()).result.workspace}
  const tree=(name:string)=>page.locator('.tree-service').filter({has:page.locator('.service-tree-row > span',{hasText:new RegExp('^'+name+'$')})})
  const picker=page.locator('.message-picker')
  await page.goto('/');const before=await snapshot()
  await upload();await expect(picker.getByRole('checkbox')).toHaveCount(181)
  await expect(picker.getByRole('button',{name:'确认导入',exact:true})).toBeDisabled()
  await picker.getByPlaceholder('搜索接口名称或消息类型').fill(' Logon ')
  await picker.getByRole('button',{name:'全选当前结果',exact:true}).click()
  await expect(picker.getByRole('checkbox',{name:'Logon (A)',exact:true})).toBeChecked()
  await picker.getByPlaceholder('搜索接口名称或消息类型').fill('newordersingle')
  await picker.getByRole('checkbox',{name:'NewOrderSingle (D)',exact:true}).check()
  await expect(picker).toContainText('已选 2 个接口')
  await picker.getByRole('button',{name:'清空选择',exact:true}).click()
  await expect(picker.getByRole('button',{name:'确认导入',exact:true})).toBeDisabled()
  await picker.getByRole('button',{name:'取消',exact:true}).click();await expect(picker).not.toBeVisible()
  expect(await snapshot()).toEqual(before)
  // Malformed XML and cancelled previews must never leave a dictionary or service behind.
  await page.locator('input[type=file][accept=".xml"]').setInputFiles({name:'bad.xml',mimeType:'text/xml',buffer:Buffer.from('<fix>')})
  await expect(page.locator('.toast.error')).toBeVisible();await expect(picker).not.toBeVisible();expect(await snapshot()).toEqual(before)
  await upload();await expect(picker.getByRole('button',{name:'确认导入',exact:true})).toBeDisabled();await expect(page.locator('.toast.error')).not.toBeVisible()
  await picker.getByPlaceholder('搜索接口名称或消息类型').fill('Logon')
  await picker.getByRole('checkbox',{name:'Logon (A)',exact:true}).check()
  await picker.getByPlaceholder('搜索接口名称或消息类型').fill('NewOrderSingle')
  await picker.getByRole('checkbox',{name:'NewOrderSingle (D)',exact:true}).check()
  await picker.getByPlaceholder('搜索接口名称或消息类型').fill('no-such-interface')
  await expect(picker).toContainText('没有匹配的消息接口');await expect(picker).toContainText('已选 2 个接口')
  await picker.getByPlaceholder('搜索接口名称或消息类型').fill('Logon')
  await page.screenshot({path:'../.local/screenshots/selective-import.png',fullPage:true})
  await page.route('**/api',async route=>{if(route.request().postDataJSON()?.method==='ImportProtocol')await route.fulfill({json:{error:'Simulated import failure'}});else await route.continue()})
  await picker.getByRole('button',{name:'确认导入',exact:true}).click();await expect(page.locator('.toast.error')).toContainText('Simulated import failure')
  await expect(picker).toBeVisible();await expect(picker).toContainText('已选 2 个接口');expect(await snapshot()).toEqual(before)
  await page.unroute('**/api');await picker.getByRole('button',{name:'确认导入',exact:true}).click();await expect(picker).not.toBeVisible()
  await expect(tree('SelectiveProtocol').locator('.tree-request-row')).toHaveCount(2)
  await expect(tree('SelectiveProtocol').locator('.tree-request-button')).toHaveText(['FIXLogon (A)','FIXNewOrderSingle (D)'])
  await tree('SelectiveProtocol').getByRole('button',{name:'FIX NewOrderSingle (D)',exact:true}).click();await expect(page.locator('.fix-form-scroll')).toContainText('Account')
  await page.getByTitle('补全消息模板 SelectiveProtocol',{exact:true}).click()
  await expect(picker.getByRole('checkbox',{name:'Logon (A)',exact:true})).toBeDisabled()
  await picker.getByPlaceholder('搜索接口名称或消息类型').fill('Heartbeat')
  await chooseInterfaces(page,['Heartbeat (0)']);await expect(tree('SelectiveProtocol').locator('.tree-request-row')).toHaveCount(3)
  // Reuse the complete dictionary for a service with one selected interface.
  await page.getByTitle('协议字典',{exact:true}).click()
  const card=page.locator('.dictionary-card').filter({hasText:'SelectiveProtocol.xml'});await expect(card).toContainText('181')
  await card.getByRole('button',{name:'创建 FIX 服务',exact:true}).click()
  await page.getByRole('dialog').getByRole('textbox').first().fill('SelectiveService')
  await page.getByRole('dialog').getByRole('button',{name:'保存服务',exact:true}).click()
  await expect(page.getByRole('dialog')).not.toBeVisible()
  await expect(tree('SelectiveService').locator('.tree-request-row')).toHaveCount(0)
  await page.getByRole('button',{name:'添加消息接口',exact:true}).click()
  await picker.getByRole('button',{name:'取消',exact:true}).click()
  await expect(tree('SelectiveService')).toBeVisible()
  await page.getByRole('button',{name:'添加消息接口',exact:true}).click();await chooseInterfaces(page,['TestRequest (1)'])
  await expect(tree('SelectiveService').locator('.tree-request-row')).toHaveCount(1)
  await expect(tree('SelectiveService').locator('.tree-request-button')).toHaveText('FIXTestRequest (1)')
  await page.reload();await expect(tree('SelectiveProtocol').locator('.tree-request-row')).toHaveCount(3);await expect(tree('SelectiveService').locator('.tree-request-row')).toHaveCount(1)
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
    await expect(received).toHaveCount(1);await expect(received.locator('code')).toHaveText(['one!']);await page.getByRole('button',{name:'会话记录',exact:true}).click();await expect(received).toHaveCount(1);await expect(received.locator('code')).toHaveText(['two!']);await page.getByRole('button',{name:'当前请求',exact:true}).click()
    await page.getByRole('button',{name:'发送报文',exact:true}).click()
    await expect(received).toHaveCount(2);await expect(received.first().locator('code')).toHaveText('tail!')
    expect(peers.size).toBe(1);await page.getByRole('button',{name:'断开',exact:true}).click();expect(errors).toEqual([])
  }finally{for(const c of peers)c.destroy();server.close()}
})


test('FIX42 dictionary controls forms, preview, wire and current-connection heartbeats',async({page})=>{
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
  const peers=new Set<net.Socket>(),received:string[]=[]
  const server=net.createServer(c=>{peers.add(c);let buffer='';c.on('close',()=>peers.delete(c));c.on('data',b=>{
    buffer+=b.toString();let match
    while((match=/\x0110=\d{3}\x01/.exec(buffer))){const end=match.index+8,raw=buffer.slice(0,end);buffer=buffer.slice(end);received.push(raw)
      if(raw.includes('\x0135=A\x01')){const reply=fix('A',1,'98=0\x01108=30\x01','FIX.4.2')+fix('1',2,'112=probe42\x01','FIX.4.2');c.write(reply.slice(0,8));setTimeout(()=>{if(!c.destroyed)c.write(reply.slice(8))},50)}
    }
  })});server.listen(0,'127.0.0.1');await once(server,'listening');const port=(server.address() as net.AddressInfo).port
  try{
    await page.goto('/')
    await page.locator('input[type=file][accept=".xml"]').setInputFiles(fileURLToPath(new URL('../../assets/fixspec/FIX42.xml',import.meta.url)))
    await expect(page.locator('.picker-description')).toContainText('FIX.4.2')
    await chooseInterfaces(page,['Logon (A)','NewOrderSingle (D)'])
    const tree=page.locator('.tree-service').filter({has:page.locator('.service-tree-row > span',{hasText:/^FIX42$/})})
    await expect(tree.locator('.tree-request-row')).toHaveCount(2)
    await tree.getByRole('button',{name:'FIX Logon (A)',exact:true}).click();await setPort(page,port);await tree.getByRole('button',{name:'FIX Logon (A)',exact:true}).click()
    await expect(page.locator('.connection-type')).toHaveText('FIX.4.2')
    await expect(page.locator('.fix-preview')).toContainText('8=FIX.4.2|')
    await expect(page.locator('.fix-form-scroll')).not.toContainText('Username')
    await page.getByRole('button',{name:'连接',exact:true}).click();await page.getByRole('button',{name:'发送报文',exact:true}).click()
    await expect(page.locator('.status-pill')).toHaveText('FIX 已登录')
    await expect.poll(()=>received.some(raw=>raw.includes('35=0\x01')&&raw.includes('112=probe42\x01'))).toBeTruthy()
    await tree.getByRole('button',{name:'FIX NewOrderSingle (D)',exact:true}).click()
    const field=(name:string)=>page.locator('.fix-field-row').filter({has:page.locator('label',{hasText:new RegExp('^\\*?'+name+'\\d')})})
    await field('ClOrdID').locator('input').fill('FIX42-ORDER')
    await field('Symbol').locator('input').fill('TEST42')
    await field('OrderQty').locator('input').fill('10')
    for(const [name,label] of [['HandlInst','1 · AUTOMATED_EXECUTION_ORDER_PRIVATE_NO_BROKER_INTERVENTION'],['Side','1 · BUY'],['OrdType','1 · MARKET']]){await field(name).locator('.n-base-selection').click();await page.getByText(label,{exact:true}).click()}
    await field('TransactTime').getByRole('button',{name:'现在',exact:true}).click()
    await expect(page.locator('.fix-preview')).toContainText('8=FIX.4.2|')
    await page.getByRole('button',{name:'发送报文',exact:true}).click()
    await expect.poll(()=>received.some(raw=>raw.includes('35=D\x01')&&raw.includes('11=FIX42-ORDER\x01'))).toBeTruthy()
    expect(received.every(raw=>raw.startsWith('8=FIX.4.2\x01'))).toBeTruthy();expect(peers.size).toBe(1)
    for(const raw of received){const bodyStart=raw.indexOf('\x01',raw.indexOf('9='))+1,tail=raw.lastIndexOf('10=');expect(Number(/\x019=(\d+)\x01/.exec(raw)![1])).toBe(Buffer.byteLength(raw.slice(bodyStart,tail)));expect(Number(raw.slice(tail+3,tail+6))).toBe(Buffer.from(raw.slice(0,tail)).reduce((sum,b)=>(sum+b)%256,0))}
    await page.screenshot({path:'../.local/screenshots/fix42-workbench.png',fullPage:true})
    for(const c of peers)c.write(fix('0',3,'','FIX.4.4'))
    await expect(page.locator('.status-pill')).toHaveText('未连接')
    await page.getByRole('button',{name:/脚本与事件/}).click();await expect(page.locator('.console-list')).toContainText('FIX 版本与绑定字典不匹配')
    await page.reload()
    await tree.getByRole('button',{name:'FIX NewOrderSingle (D)',exact:true}).click()
    await expect(page.locator('.connection-type')).toHaveText('FIX.4.2');await expect(page.locator('.fix-preview')).toContainText('11=FIX42-ORDER|')
    // Existing 4.4 service must retain its version in the same workspace.
    await page.getByRole('complementary').getByRole('button',{name:'FIX 登录 Logon',exact:true}).click()
    await expect(page.locator('.connection-type')).toHaveText('FIX.4.4');await expect(page.locator('.fix-preview')).toContainText('8=FIX.4.4|')
    const serviceCount=await page.locator('.tree-service').count()
    await page.locator('input[type=file][accept=".xml"]').setInputFiles({name:'FIX50.xml',mimeType:'text/xml',buffer:Buffer.from('<fix type="FIX" major="5" minor="0"><messages><message name="Heartbeat" msgtype="0"/></messages></fix>')})
    await expect(page.locator('.toast.error')).toContainText('FIX 5.x/FIXT 会话暂不支持');await expect(page.locator('.message-picker')).not.toBeVisible();await expect(page.locator('.tree-service')).toHaveCount(serviceCount)
    expect(errors).toEqual([])
  }finally{for(const c of peers)c.destroy();server.close()}
})


test('new FIX service uses selected built-in protocol without XML, import or mandatory templates',async({page})=>{
  const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
  const peers=new Set<net.Socket>(),received:string[]=[]
  const server=net.createServer(c=>{peers.add(c);let buffer='';c.on('close',()=>peers.delete(c));c.on('data',b=>{buffer+=b.toString();let match;while((match=/\x0110=\d{3}\x01/.exec(buffer))){const end=match.index+8,raw=buffer.slice(0,end);buffer=buffer.slice(end);received.push(raw);if(raw.includes('35=A\x01'))c.write(fix('A',1,'98=0\x01108=30\x01','FIX.4.2')+fix('1',2,'112=builtin-probe\x01','FIX.4.2'))}})});server.listen(0,'127.0.0.1');await once(server,'listening');const port=(server.address() as net.AddressInfo).port
  try{
    // End earlier isolated test sessions before replacing this test workspace.
    const state=await (await page.request.post('/api',{data:{method:'Snapshot',args:[0]}})).json()
    for(const id of Object.keys(state.result.sessions))await page.request.post('/api',{data:{method:'Disconnect',args:[id]}})
    // Empty protocol library is the actual regression precondition.
    const reset=await page.request.post('/api',{data:{method:'Save',args:[{version:1,services:[],requests:[],dictionaries:[],functions:[]}]}});expect((await reset.json()).error).toBeUndefined()
    await page.goto('/')
    let fileChoosers=0;page.on('filechooser',()=>fileChoosers++)
    const imports:string[]=[];page.on('request',r=>{if(r.method()==='POST'&&r.url().endsWith('/api')){const method=r.postDataJSON()?.method;if(method==='ImportProtocol'||method==='ImportDictionary')imports.push(method)}})
    await page.getByTitle('新建',{exact:true}).click();await page.getByText('FIX 协议服务',{exact:true}).click()
    const modal=page.getByRole('dialog')
    await expect(modal).toContainText('协议版本');await expect(modal).not.toContainText('协议字典')
    await expect(modal.getByRole('button',{name:'导入',exact:true})).toHaveCount(0)
    await modal.getByRole('textbox').first().fill('Builtin FIX42')
    await modal.locator('.n-input-number input').first().fill(String(port))
    await modal.locator('.fix-protocol-select').click();await page.getByText('FIX.4.2（内置标准协议）',{exact:true}).click()
    await page.screenshot({path:'../.local/screenshots/new-fix-protocol.png',fullPage:true})
    // Failed save preserves the draft and permits retry.
    await page.route('**/api',async route=>{if(route.request().postDataJSON()?.method==='SaveService')await route.fulfill({json:{error:'Service save failure'}});else await route.continue()})
    await modal.getByRole('button',{name:'保存服务',exact:true}).click();await expect(page.locator('.toast.error')).toContainText('Service save failure');await expect(modal.getByRole('textbox').first()).toHaveValue('Builtin FIX42')
    await page.unroute('**/api');await modal.getByRole('button',{name:'保存服务',exact:true}).click()
    await expect(modal).not.toBeVisible();await expect(page.locator('.message-picker')).not.toBeVisible();await expect(page.locator('.connection-type')).toHaveText('FIX.4.2')
    const snapshot=async()=>{const response=await page.request.post('/api',{data:{method:'Snapshot',args:[0]}});return (await response.json()).result.workspace}
    let w=await snapshot();expect(w.services).toHaveLength(1);expect(w.dictionaries).toHaveLength(0);expect(w.requests).toHaveLength(0);expect(w.services[0].dictionaryId).toBe('');expect(w.services[0].fixVersion).toBe('FIX.4.2')
    await page.reload();await page.locator('.service-tree-row').filter({hasText:'Builtin FIX42'}).click()
    await expect(page.locator('.connection-type')).toHaveText('FIX.4.2')
    await page.getByRole('button',{name:'添加消息接口',exact:true}).click();await chooseInterfaces(page,['Logon (A)','Heartbeat (0)'])
    await expect(page.locator('.fix-preview')).toContainText('8=FIX.4.2|')
    await page.getByRole('button',{name:'连接',exact:true}).click();await page.getByRole('button',{name:'发送报文',exact:true}).click()
    await expect(page.locator('.status-pill')).toHaveText('FIX 已登录');await expect.poll(()=>received.some(raw=>raw.startsWith('8=FIX.4.2\x01')&&raw.includes('35=0\x01')&&raw.includes('112=builtin-probe\x01'))).toBeTruthy()
    expect(peers.size).toBe(1);expect(received.every(raw=>raw.startsWith('8=FIX.4.2\x01'))).toBeTruthy()
    await page.getByRole('button',{name:'断开',exact:true}).click()
    w=await snapshot();expect(w.dictionaries).toHaveLength(0);expect(w.requests).toHaveLength(2);expect(fileChoosers).toBe(0);expect(imports).toEqual([])
    await page.reload();await page.getByRole('complementary').getByRole('button',{name:'FIX Logon (A)',exact:true}).click();await expect(page.locator('.fix-preview')).toContainText('8=FIX.4.2|')
    expect(errors).toEqual([])
  }finally{for(const c of peers)c.destroy();server.close()}
})


test('request histories, schema editing, expanded rows and editable managed FIX text',async({page})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
 const peers=new Set<net.Socket>(),sent:string[]=[]
 const server=net.createServer(c=>{peers.add(c);c.on('close',()=>peers.delete(c));let buffer='';c.on('data',b=>{buffer+=b.toString();let match;while((match=/\x0110=\d{3}\x01/.exec(buffer))){const end=match.index+8;sent.push(buffer.slice(0,end));buffer=buffer.slice(end);if(sent.length===2){c.write(fix('8',1,'11=B-ORDER\x019001=reply-B\x01','FIX.4.2'));c.write(fix('8',2,'11=A-ORDER\x019001=reply-A\x01','FIX.4.2'));c.write(fix('0',3,'','FIX.4.2'))}}})});server.listen(0,'127.0.0.1');await once(server,'listening')
 const port=(server.address() as net.AddressInfo).port
 const call=async(method:string,...args:any[])=>{const response=await page.request.post('/api',{data:{method,args}});const body=await response.json();expect(body.error).toBeFalsy();return body.result}
 try{
  await page.goto('/');const snapshot=await call('Snapshot',0);const w=snapshot.workspace
  const svc={...w.services[0],id:'history-fixture',name:'请求隔离测试',protocol:'fix',fixVersion:'FIX.4.2',dictionaryId:'',host:'127.0.0.1',port,sender:'FixClient',target:'Fix',heartbeat:30,autoLogon:false,resetOnLogon:false}
  const nodes=[{kind:'field',name:'OptionalProbe',number:9005,type:'STRING',required:false},{kind:'field',name:'ClOrdID',number:11,type:'STRING',required:true},{kind:'field',name:'BusinessValue',number:9001,type:'STRING',required:true},{kind:'group',name:'Items',number:9002,type:'NUMINGROUP',required:false,children:[{kind:'field',name:'ItemID',number:9003,type:'STRING',required:true}]}]
  const request=(id:string,order:string)=>({id,serviceId:svc.id,name:id==='history-a'?'隔离请求 A':'隔离请求 B',data:'',encoding:'text',preScript:'',postScript:`console.log("post-${id}");`,requestFunctionId:'',responseFunctionId:'',messageType:'D',values:{OptionalProbe:'optional',ClOrdID:order,BusinessValue:'original',Items:[{ItemID:'first'},{ItemID:'second'}]},rawMode:false,customNodes:nodes,description:'独立请求定义与响应关联'})
  w.services.push(svc);w.requests.push(request('history-a','A-ORDER'),request('history-b','B-ORDER'));await call('Save',w);await page.reload()
  const open=async(name:string)=>{await page.getByRole('complementary').getByRole('button',{name:'FIX '+name,exact:true}).click()}
  await open('隔离请求 A')
  await page.getByRole('button',{name:'文档',exact:true}).click();await expect(page.locator('.request-document')).toContainText('BusinessValue');await expect(page.locator('.request-document')).toContainText('独立请求定义')
  await page.getByRole('button',{name:'编辑',exact:true}).click();await page.locator('.request-definition > .schema-editor > .schema-add').getByRole('button',{name:'＋ 添加字段',exact:true}).click();await expect(page.getByTitle('删除字段 Field9004',{exact:true})).toBeVisible()
  await page.locator('.request-definition').getByRole('textbox',{name:'字段名称 2',exact:true}).fill('BusinessLabel');await page.getByTitle('删除字段 Field9004',{exact:true}).click();await expect(page.getByTitle('删除字段 Field9004',{exact:true})).toHaveCount(0)
  await page.screenshot({path:'../.local/screenshots/request-schema-editor.png',fullPage:true});await page.getByRole('button',{name:'调试',exact:true}).click()
  const rows=page.locator('.fix-field-row');await expect(rows.nth(0).locator('label')).toContainText('ClOrdID');await expect(rows.nth(1).locator('label')).toContainText('BusinessLabel');const first=await rows.nth(0).boundingBox(),second=await rows.nth(1).boundingBox();expect(second!.y).toBeGreaterThan(first!.y+first!.height-1)
  const group=page.locator('.group-box');await expect(group.locator('.group-row')).toHaveCount(2);await group.locator(':scope > summary').click();await expect(group.locator('.group-row').first()).not.toBeVisible();await group.locator(':scope > summary').click();await expect(group.locator('.group-row').first()).toBeVisible()
  await page.getByRole('button',{name:'连接',exact:true}).click()
  await expect(page.locator('.fix-preview code')).toContainText('9001=original');const wirePreview=await page.locator('.fix-preview code').textContent();expect(wirePreview!.indexOf('9005=optional')).toBeLessThan(wirePreview!.indexOf('11=A-ORDER'));expect((await page.locator('.fix-preview').boundingBox())!.height).toBeLessThan(65);await expect(page.locator('.fix-preview .code-editor')).toHaveCount(0);await page.getByRole('checkbox',{name:'原始报文',exact:true}).click();const draft=page.locator('.request-editor-card .code-editor');await expect(draft).toContainText('9001=original');expect((await draft.boundingBox())!.height).toBeGreaterThanOrEqual(220)
  await page.getByRole('checkbox',{name:'原始报文',exact:true}).uncheck();await page.locator('.fix-field-row').filter({hasText:'BusinessLabel'}).locator('input').fill('refreshed');await page.getByRole('checkbox',{name:'原始报文',exact:true}).click();await expect(draft).toContainText('9001=refreshed');const input=draft.locator('textarea');await input.focus();await input.press('ControlOrMeta+A');await page.keyboard.insertText('8=FIX.4.2|9=0|35=D|34=999|52=20000101-00:00:00|11=A-ORDER|9001=edited|9002=2|9003=first|9003=second|10=000|')
  await page.getByRole('checkbox',{name:'原始报文',exact:true}).uncheck();await expect(page.locator('.fix-preview code')).toContainText('9001=refreshed');await page.getByRole('checkbox',{name:'原始报文',exact:true}).click();await expect(draft).toContainText('9001=edited');await page.getByRole('button',{name:'发送报文',exact:true}).click();await expect.poll(()=>sent.length).toBe(1)
  expect(sent[0]).toContain('9001=edited\x01');expect(sent[0]).toContain('9002=2\x019003=first\x019003=second\x01');expect(sent[0]).toContain('34=1\x01');expect(sent[0]).not.toContain('20000101')
  await open('隔离请求 B');await expect(page.locator('.message-entry')).toHaveCount(0);await page.getByRole('button',{name:'发送报文',exact:true}).click();await expect.poll(()=>sent.length).toBe(2)
  await expect(page.locator('.message-entry')).toHaveCount(2);await expect(page.locator('.response-text')).toContainText('reply-B');await expect(page.locator('.message-list')).not.toContainText('reply-A')
  await page.getByRole('button',{name:'FIX 字段',exact:true}).click();await expect(page.locator('.decoded-table')).toContainText('BusinessValue');await expect(page.locator('.decoded-table')).toContainText('reply-B')
  await open('隔离请求 A');await expect(page.locator('.message-entry')).toHaveCount(2);await expect(page.locator('.decoded-table')).toContainText('reply-A');await expect(page.locator('.decoded-table')).toContainText('BusinessLabel');await expect(page.locator('.message-list')).not.toContainText('reply-B')
  await page.getByRole('button',{name:/脚本与事件/}).click();await expect(page.locator('.console-list')).toContainText('post-history-a');await expect(page.locator('.console-list')).not.toContainText('post-history-b')
  await page.getByRole('button',{name:/收发记录/}).click();await page.getByRole('button',{name:'会话记录',exact:true}).click();await expect(page.locator('.message-entry')).toHaveCount(1);await expect(page.locator('.decoded-table')).toContainText('MsgType')
  await page.getByRole('button',{name:'当前请求',exact:true}).click();await page.getByRole('button',{name:'清空',exact:true}).click();await expect(page.locator('.message-entry')).toHaveCount(0)
  await open('隔离请求 B');await expect(page.locator('.message-entry')).toHaveCount(2)
  await page.getByRole('button',{name:'断开',exact:true}).click();await page.screenshot({path:'../.local/screenshots/request-scoped-debug.png',fullPage:true});await page.reload();await open('隔离请求 A');await expect(page.locator('.request-editor-card .code-editor')).toContainText('9001=edited');expect(errors).toEqual([])
 }finally{for(const c of peers)c.destroy();server.close()}
})


test('periodic TCP sending without post scripts and visible FIX automatic heartbeat',async({page})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
 const tcpPeers=new Set<net.Socket>(),tcpReceived:string[]=[]
 const echo=net.createServer(c=>{tcpPeers.add(c);c.on('close',()=>tcpPeers.delete(c));c.on('data',b=>{tcpReceived.push(b.toString());c.write(b)})});echo.listen(0,'127.0.0.1');await once(echo,'listening')
 const fixPeers=new Set<net.Socket>(),fixReceived:string[]=[]
 const server=net.createServer(c=>{fixPeers.add(c);c.on('close',()=>fixPeers.delete(c));let seq=1,buffer='';c.on('data',b=>{buffer+=b.toString();let match;while((match=/\x0110=\d{3}\x01/.exec(buffer))){const end=match.index+8,raw=buffer.slice(0,end);buffer=buffer.slice(end);fixReceived.push(raw);if(raw.includes('\x0135=A\x01'))c.write(fix('A',seq++,'98=0\x01108=1\x01','FIX.4.2'));else if(raw.includes('\x0135=1\x01')){const id=raw.match(/\x01112=([^\x01]+)/)?.[1]||'';c.write(fix('0',seq++,'112='+id+'\x01','FIX.4.2'))}else if(raw.includes('\x0135=0\x01'))c.write(fix('0',seq++,'','FIX.4.2'))}})});server.listen(0,'127.0.0.1');await once(server,'listening')
 const call=async(method:string,...args:any[])=>{const response=await page.request.post('/api',{data:{method,args}});const body=await response.json();expect(body.error).toBeFalsy();return body.result}
 try{
  await page.goto('/');const snapshot=await call('Snapshot',0),w=snapshot.workspace
  const base={...w.services[0],host:'127.0.0.1',autoLogon:false,resetOnLogon:false}
  const tcp={...base,id:'periodic-tcp',name:'定时TCP',protocol:'tcp',port:(echo.address() as net.AddressInfo).port,framing:'delimiter',delimiter:'\n'}
  const fixed={...base,id:'periodic-fix',name:'心跳FIX42',protocol:'fix',fixVersion:'FIX.4.2',dictionaryId:'',port:(server.address() as net.AddressInfo).port,sender:'FixClient',target:'Fix',heartbeat:30}
  const r={id:'periodic-pulse',serviceId:tcp.id,name:'周期心跳',data:'ping\n',encoding:'text',preScript:'return data.replace("ping","heartbeat");',postScript:'',requestFunctionId:'',responseFunctionId:'',messageType:'',values:{},rawMode:false}
  const logon={...r,id:'periodic-logon',serviceId:fixed.id,name:'心跳登录',data:'',messageType:'A',preScript:'',values:{EncryptMethod:'0',HeartBtInt:'1'}}
  const hb={...logon,id:'periodic-fix-heartbeat',name:'手动心跳',messageType:'0',values:{}}
  w.services.push(tcp,fixed);w.requests.push(r,logon,hb);await call('Save',w);await page.reload()
  await page.getByRole('complementary').getByRole('button',{name:'TCP 周期心跳',exact:true}).click();await page.getByRole('button',{name:'连接',exact:true}).click();await page.getByRole('button',{name:/定时任务/}).click()
  await page.getByRole('textbox',{name:'定时发送间隔（秒）',exact:true}).fill('1');await page.getByRole('button',{name:'启动定时发送',exact:true}).click()
  await expect(page.locator('.task-row')).toContainText('定时发送 · 周期心跳');await expect.poll(()=>tcpReceived.length).toBeGreaterThanOrEqual(2);expect(tcpReceived.every(text=>text==='heartbeat\n')).toBeTruthy();expect(tcpPeers.size).toBe(1)
  await page.getByRole('button',{name:'停止定时发送',exact:true}).click();await expect(page.locator('.task-row')).toHaveCount(0)
  const stopped=tcpReceived.length;await expect.poll(async()=>{await new Promise(resolve=>setTimeout(resolve,1100));return tcpReceived.length}).toBe(stopped)
  await page.getByRole('button',{name:/收发记录/}).click();await expect(page.locator('.message-entry').filter({has:page.locator('.send-icon')})).toHaveCount(stopped)
  await page.getByRole('button',{name:/定时任务/}).click();await page.getByRole('button',{name:'启动定时发送',exact:true}).click();await page.getByRole('button',{name:'断开',exact:true}).click();await expect(page.locator('.task-row')).toHaveCount(0)
  await page.getByRole('complementary').getByRole('button',{name:'FIX 心跳登录',exact:true}).click();await page.getByRole('button',{name:'连接',exact:true}).click();await page.getByRole('button',{name:/定时任务/}).click();await expect(page.locator('.heartbeat-task')).toContainText('等待 FIX 登录')
  await page.getByRole('button',{name:'发送报文',exact:true}).click();await expect(page.getByRole('button',{name:/收发记录/})).toHaveClass(/active/);await page.getByRole('button',{name:/定时任务/}).click();await expect(page.locator('.heartbeat-task')).toContainText('运行中');await expect(page.locator('.heartbeat-task')).toContainText('1 秒空闲间隔')
  await expect.poll(()=>fixReceived.some(raw=>raw.includes('\x0135=0\x01')),{timeout:6000}).toBeTruthy();await expect(page.locator('.heartbeat-task')).not.toContainText('尚未发送');await page.screenshot({path:'../.local/screenshots/heartbeat-tasks.png',fullPage:true})
  await page.getByRole('button',{name:'查看心跳记录',exact:true}).click();await expect(page.locator('.message-entry').filter({has:page.locator('.send-icon')})).not.toHaveCount(0);await expect(page.locator('.message-list')).toContainText('35=0')
  await page.getByRole('complementary').getByRole('button',{name:'FIX 手动心跳',exact:true}).click();await expect(page.locator('.fix-preview code')).toContainText('35=0');await page.getByRole('checkbox',{name:'原始报文',exact:true}).click();await expect(page.locator('.request-editor-card .code-editor')).toContainText('35=0');await expect(page.getByRole('checkbox',{name:'跳过自动计算（异常测试）',exact:true})).not.toBeChecked()
  await page.screenshot({path:'../.local/screenshots/raw-message-from-fields.png',fullPage:true})
  await page.getByRole('button',{name:'断开',exact:true}).click();expect(errors).toEqual([])
 }finally{for(const c of tcpPeers)c.destroy();for(const c of fixPeers)c.destroy();echo.close();server.close()}
})
