import type { Dictionary, FixNode } from '../../types'
export interface DecodedRow { tag: number; name: string; value: string; description: string; path: string }
// Preserve duplicate tags. Group paths are derived from dictionary order and delimiters.
export function decodeFix(raw: string, dictionary?: Dictionary): DecodedRow[] {
  const fields = new Map(dictionary?.fields.map(f=>[f.number,f])||[])
  const pairs=raw.split('\x01').filter(Boolean).map(s=>{const i=s.indexOf('=');return {tag:Number(s.slice(0,i)),value:s.slice(i+1)}})
  const paths=new Map<number,string>()
  const type=pairs.find(p=>p.tag===35)?.value
  const schema=dictionary?.messages.find(m=>m.type===type)
  const flattened=(nodes:FixNode[]):FixNode[]=>nodes.flatMap(n=>n.kind==='component'?flattened(n.children||[]):[n])
  function group(node:FixNode,index:number,path:string):number {
    paths.set(index,path)
    const count=Number(pairs[index]!.value)
    const children=flattened(node.children||[]),delimiter=children[0]?.number
    const byTag=new Map(children.map(n=>[n.number,n]))
    index++
    if(!Number.isInteger(count)||count<0||count>1000||delimiter==null)return index
    for(let row=0;row<count&&index<pairs.length;row++){
      if(pairs[index]!.tag!==delimiter)break
      const rowPath=`${path}${node.name}[${row+1}].`
      let first=true
      while(index<pairs.length){
        const pair=pairs[index]!,child=byTag.get(pair.tag)
        if(!child||!first&&pair.tag===delimiter)break
        paths.set(index,rowPath)
        index=child.kind==='group'?group(child,index,rowPath):index+1
        first=false
      }
    }
    return index
  }
  if(schema){
    const roots=new Map(flattened(schema.nodes).map(n=>[n.number,n]))
    for(let i=0;i<pairs.length;){const n=roots.get(pairs[i]!.tag);i=n?.kind==='group'?group(n,i,''):i+1}
  }
  return pairs.map((p,index)=>{const f=fields.get(p.tag);return {tag:p.tag,name:f?.name||'未知字段',value:p.value,description:f?.enums?.find(e=>e.value===p.value)?.description||'',path:paths.get(index)||''}})
}
