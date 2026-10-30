<script setup lang="ts">
import { NInput, NSelect, NButton, NCheckbox } from 'naive-ui'
import { computed } from 'vue'
import type { FixNode } from '../../types'
defineOptions({name:'FixFields'})
const props=defineProps<{nodes:FixNode[];values:Record<string,any>;showOptional:boolean;depth?:number;version?:string}>()
const recommended=['Symbol','SecurityExchange','Price','OrderQty','Account','ClOrdID','Username','Password','Text','TestReqID','TimeInForce']
const visible=computed(()=>props.nodes.filter(n=>n.required||props.showOptional||props.values[n.name]!==undefined||recommended.includes(n.name)||n.kind==='component'&&hasValue(n)).sort((a,b)=>Number(b.required)-Number(a.required)))
function hasValue(n:FixNode):boolean{return (n.children||[]).some(c=>props.values[c.name]!==undefined||c.kind==='component'&&hasValue(c))}
function update(name:string,v:any){if(v==null||v==='')delete props.values[name];else props.values[name]=v}
function updateEnum(n:FixNode,v:unknown){update(n.name,Array.isArray(v)?v.join(' '):v)}
function enabled(n:FixNode){return n.required||!!props.values['__component_'+n.name]||hasValue(n)}
function toggle(n:FixNode,v:boolean){if(v)props.values['__component_'+n.name]=true;else{delete props.values['__component_'+n.name];clear(n.children||[],props.values)}}
function clear(ns:FixNode[],v:Record<string,any>){for(const n of ns){delete v[n.name];delete v['__component_'+n.name];if(n.kind==='component')clear(n.children||[],v)}}
function now(n:FixNode){const iso=new Date().toISOString();const end=['FIX.4.0','FIX.4.1'].includes(props.version||'')?19:23;update(n.name,['UTCTIMESTAMP','TIME'].includes(n.type)?iso.slice(0,10).replace(/-/g,'')+'-'+iso.slice(11,end):n.type==='UTCTIMEONLY'?iso.slice(11,end):iso.slice(0,10).replace(/-/g,''))}
</script>
<template>
  <div class="fix-fields" :class="{'nested-fields':depth}">
    <div v-if="!depth" class="field-table-head"><span>字段名称</span><span>Tag</span><span>类型</span><span>字段值</span></div>
    <template v-for="n in visible" :key="n.name">
      <section v-if="n.kind==='component'" class="component-box">
        <div class="component-title"><span>{{ n.name }}</span><span v-if="n.required" class="required">必需组件</span><n-checkbox v-else :checked="enabled(n)" @update:checked="v=>toggle(n,v)">启用</n-checkbox></div>
        <FixFields v-if="enabled(n)" :nodes="n.children||[]" :values="values" :show-optional="showOptional" :version="version" :depth="(depth||0)+1"/>
      </section>
      <details v-else-if="n.kind==='group'" class="component-box group-box" open>
        <summary class="component-title"><span>{{ n.name }} <small>#{{n.number}} · 重复组 · {{(values[n.name]||[]).length}} 条 · 点击展开设置</small></span><n-button size="tiny" secondary @click.prevent.stop="(values[n.name]??=[]).push({})">＋ 添加记录</n-button></summary>
        <details v-for="(row,index) in (values[n.name]||[])" :key="index" class="group-row" open><summary class="group-row-title">记录 {{Number(index)+1}}<button @click.prevent.stop="values[n.name].splice(index,1)">删除</button></summary><FixFields :nodes="n.children||[]" :values="row" :show-optional="showOptional" :version="version" :depth="(depth||0)+1"/></details>
      </details>
      <div v-else class="fix-field-row">
        <label :title="n.name"><span v-if="n.required" class="required">*</span>{{n.name}}<small class="field-label-meta">{{n.number}} · {{n.type}}</small></label>
        <code class="field-tag">{{n.number}}</code><span class="field-type">{{n.type}}</span>
        <div class="field-control"><n-select v-if="n.enums?.length" size="small" filterable clearable :multiple="n.type.startsWith('MULTIPLE')" :value="n.type.startsWith('MULTIPLE')?(values[n.name]?String(values[n.name]).split(' '):[]):values[n.name]??null" :options="n.enums.map(e=>({label:e.value+' · '+e.description,value:e.value}))" @update:value="v=>updateEnum(n,v)"/>
        <n-select v-else-if="n.type==='BOOLEAN'" size="small" clearable :value="values[n.name]??null" :options="[{label:'Y · 是',value:'Y'},{label:'N · 否',value:'N'}]" @update:value="v=>update(n.name,v)"/>
        <n-input v-else size="small" :type="/password/i.test(n.name)?'password':'text'" show-password-on="click" :value="values[n.name]??''" :placeholder="n.required?'必填':'留空则不发送'" @update:value="v=>update(n.name,v)"/>
        <n-button v-if="['UTCTIMESTAMP','TIME','UTCTIMEONLY','UTCDATEONLY','UTCDATE','DATE','LOCALMKTDATE'].includes(n.type)" size="tiny" quaternary @click="now(n)">现在</n-button>
        <n-button v-if="n.name==='ClOrdID'" size="tiny" quaternary @click="update(n.name,'TC'+Date.now())">生成</n-button></div>
      </div>
    </template>
    <div v-if="!visible.length" class="muted small">无必填字段；可打开“显示可选字段”进行配置。</div>
  </div>
</template>
