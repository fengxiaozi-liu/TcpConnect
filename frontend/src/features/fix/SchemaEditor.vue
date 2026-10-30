<script setup lang="ts">
import { NInput,NInputNumber,NSelect,NCheckbox,NButton } from 'naive-ui'
import { reactive } from 'vue'
import type { FixNode } from '../../types'
defineOptions({name:'SchemaEditor'})
const props=defineProps<{nodes:FixNode[];editable?:boolean;depth?:number;root?:FixNode[];values?:Record<string,any>[]}>()
const types=['STRING','CHAR','INT','NUMINGROUP','FLOAT','PRICE','QTY','AMT','PERCENTAGE','BOOLEAN','UTCTIMESTAMP','UTCTIMEONLY','UTCDATEONLY','LOCALMKTDATE','MULTIPLEVALUESTRING','MULTIPLECHARVALUE','MULTIPLESTRINGVALUE','DATA','LENGTH','SEQNUM','CURRENCY','COUNTRY','EXCHANGE','MONTHYEAR','DAYOFMONTH']
function add(group=false){const used=new Set<number>();function collect(nodes:FixNode[]){for(const n of nodes){if(n.number)used.add(n.number);if(n.children)collect(n.children)}};collect(props.root||props.nodes);let number=9001;while(used.has(number))number++;let child=number+1;while(used.has(child))child++;props.nodes.push({kind:group?'group':'field',name:(group?'Rows':'Field')+number,number,type:group?'NUMINGROUP':'STRING',required:false,...(group?{children:[{kind:'field' as const,name:'RowID'+child,number:child,type:'STRING',required:true}]}:{})})}
const enumDrafts=reactive(new Map<FixNode,string>())
function enumText(n:FixNode){return enumDrafts.get(n)??(n.enums||[]).map(e=>e.value+'='+e.description).join('\n')}
function enums(n:FixNode,text:string){enumDrafts.set(n,text);n.enums=text.split('\n').filter(Boolean).map(line=>{const i=line.indexOf('=');return i<0?{value:line,description:line}:{value:line.slice(0,i),description:line.slice(i+1)}})}
function rename(n:FixNode,name:string){if(name===n.name||props.nodes.some(other=>other!==n&&other.name===name)||(props.values||[]).some(v=>Object.hasOwn(v,name)))return;for(const values of props.values||[]){if(Object.hasOwn(values,n.name)){values[name]=values[n.name];delete values[n.name]};if(n.kind==='component'&&Object.hasOwn(values,'__component_'+n.name)){values['__component_'+name]=values['__component_'+n.name];delete values['__component_'+n.name]}};n.name=name}
function remove(n:FixNode,index:number){function clear(nodes:FixNode[],values:Record<string,any>){for(const node of nodes){delete values[node.name];delete values['__component_'+node.name];if(node.kind==='component')clear(node.children||[],values)}};for(const values of props.values||[])clear([n],values);props.nodes.splice(index,1)}
function childValues(n:FixNode){return n.kind==='group'?(props.values||[]).flatMap(v=>Array.isArray(v[n.name])?v[n.name]:[]):props.values}
function move(index:number,offset:number){const target=index+offset;if(target<0||target>=props.nodes.length)return;const [node]=props.nodes.splice(index,1);props.nodes.splice(target,0,node!)}
</script>
<template>
 <div class="schema-editor" :class="{'schema-nested':depth}">
  <div v-if="!depth" class="schema-columns"><span>字段 / 分组</span><span>Tag</span><span>类型</span><span>必填</span><span>说明 / 设置</span></div>
  <div v-for="(n,i) in nodes" :key="i" class="schema-node">
   <div class="schema-line">
    <n-input v-if="editable" :value="n.name" @update:value="name=>rename(n,name)" size="small" placeholder="字段名" :input-props="{'aria-label':'字段名称 '+i}"/><strong v-else>{{n.name}}<small v-if="n.kind!=='field'"> · {{n.kind==='group'?'重复组':'组件'}}</small></strong>
    <n-input-number v-if="editable&&n.kind!=='component'" v-model:value="n.number" size="small" :min="1" :show-button="false" :input-props="{'aria-label':'字段Tag '+i}"/><code v-else>{{n.number||'—'}}</code>
    <n-select v-if="editable&&n.kind!=='component'" v-model:value="n.type" size="small" filterable tag :options="types.map(value=>({label:value,value}))"/><span v-else>{{n.type||'组件'}}</span>
    <n-checkbox v-if="editable" v-model:checked="n.required"/><span v-else>{{n.required?'是':'否'}}</span>
    <div class="schema-actions"><span v-if="!editable">{{n.description||'—'}}</span><template v-else><n-button size="tiny" :disabled="i===0" @click="move(i,-1)" title="上移字段">↑</n-button><n-button size="tiny" :disabled="i===nodes.length-1" @click="move(i,1)" title="下移字段">↓</n-button><n-button size="tiny" type="error" @click="remove(n,i)" :title="'删除字段 '+n.name">删除</n-button></template></div>
   </div>
   <details v-if="editable||n.enums?.length||n.children?.length" class="schema-details" :open="n.kind!=='field'">
    <summary>{{n.kind==='field'?'展开设置 / 枚举':'展开设置 · '+(n.children?.length||0)+' 个子字段'}}</summary>
    <template v-if="editable"><n-input v-model:value="n.description" size="small" placeholder="字段说明"/><n-input v-if="n.kind==='field'" :value="enumText(n)" type="textarea" placeholder="枚举：每行 值=说明；留空不限制" :autosize="{minRows:2,maxRows:6}" @update:value="text=>enums(n,text)"/></template>
    <div v-else-if="n.enums?.length" class="schema-enums"><span v-for="e in n.enums" :key="e.value"><code>{{e.value}}</code> {{e.description}}</span></div>
    <SchemaEditor v-if="n.children" :nodes="n.children" :editable="editable" :depth="(depth||0)+1" :root="root||nodes" :values="childValues(n)"/>
   </details>
  </div>
  <div v-if="editable" class="schema-add"><n-button size="small" @click="add()">＋ 添加字段</n-button><n-button size="small" @click="add(true)">＋ 添加重复组</n-button></div>
  <div v-if="!nodes.length" class="inline-note">当前结构没有业务字段。</div>
 </div>
</template>
