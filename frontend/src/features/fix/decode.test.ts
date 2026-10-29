import { describe, it, expect } from 'vitest'
import { decodeFix } from './decode'
import type { Dictionary } from '../../types'
describe('FIX dictionary rendering data',()=>{
  it('preserves repeated tags and assigns group rows',()=>{
    const dictionary={fields:[{number:35,name:'MsgType',type:'STRING'},{number:453,name:'NoPartyIDs',type:'NUMINGROUP'},{number:448,name:'PartyID',type:'STRING'},{number:452,name:'PartyRole',type:'INT',enums:[{value:'1',description:'EXECUTING_FIRM'}]}],messages:[{name:'Test',type:'U',category:'app',nodes:[{kind:'group',name:'NoPartyIDs',number:453,required:false,type:'NUMINGROUP',children:[{kind:'field',name:'PartyID',number:448,required:true,type:'STRING'},{kind:'field',name:'PartyRole',number:452,required:false,type:'INT'}]}]}]} as Dictionary
    const result=decodeFix('35=U\x01453=2\x01448=A\x01452=1\x01448=B\x01452=1\x01',dictionary)
    expect(result.filter(r=>r.tag===448).map(r=>r.path)).toEqual(['NoPartyIDs[1].','NoPartyIDs[2].'])
    expect(result.find(r=>r.tag===452)?.description).toBe('EXECUTING_FIRM')
  })
  it('does not discard unknown custom tags',()=>{expect(decodeFix('35=U\x019999=value=with=equals\x01')[1]).toMatchObject({tag:9999,value:'value=with=equals',name:'未知字段'})})
  it('keeps nested groups inside their parent row',()=>{
    const field=(name:string,number:number)=>({kind:'field',name,number,required:false,type:'STRING'})
    const dictionary={fields:[],messages:[{type:'U',nodes:[{kind:'component',children:[{kind:'group',name:'Rows',number:100,children:[field('ID',101),{kind:'group',name:'Children',number:102,children:[field('ChildID',103)]},field('End',104)]}]}]}]} as unknown as Dictionary
    const result=decodeFix('35=U\x01100=2\x01101=A\x01102=2\x01103=a\x01103=b\x01104=end\x01101=B\x01104=end\x01',dictionary)
    expect(result.filter(r=>r.tag===103).map(r=>r.path)).toEqual(['Rows[1].Children[1].','Rows[1].Children[2].'])
    expect(result.filter(r=>r.tag===101).map(r=>r.path)).toEqual(['Rows[1].','Rows[2].'])
  })
})
