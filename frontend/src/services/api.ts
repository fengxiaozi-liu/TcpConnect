declare global { interface Window { go?: { interfaces?: { Bridge?: Record<string, (...args: any[]) => Promise<any>> } } } }
export async function api<T = void>(method: string, ...args: any[]): Promise<T> {
  const native = window.go?.interfaces?.Bridge?.[method]
  if (native) return native(...args)
  const response = await fetch('/api', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ method, args }) })
  if (!response.ok) throw new Error(`本机服务不可用 (${response.status})`)
  const json = await response.json()
  if (json.error) throw new Error(json.error)
  return json.result as T
}
export const uid = () => crypto.randomUUID()
export function download(name: string, content: string) { const url=URL.createObjectURL(new Blob([content],{type:'application/json;charset=utf-8'})); const a=document.createElement('a'); a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000) }
