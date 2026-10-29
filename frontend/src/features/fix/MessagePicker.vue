<script setup lang="ts">
import { computed, ref } from 'vue'
import { NModal, NInput, NCheckbox, NCheckboxGroup, NButton, NEmpty } from 'naive-ui'
import type { FixMessage } from '../../types'

const props=withDefaults(defineProps<{title:string;source:string;messages:FixMessage[];existingTypes?:string[];loading?:boolean;confirmText:string}>(),{existingTypes:()=>[],loading:false})
const emit=defineEmits<{cancel:[];confirm:[types:string[]]}>()
const query=ref(''),selected=ref<string[]>([])
const existing=computed(()=>new Set(props.existingTypes))
const visible=computed(()=>{const key=query.value.trim().toLowerCase();return props.messages.filter(m=>!key||m.name.toLowerCase().includes(key)||m.type.toLowerCase().includes(key))})
function selectVisible(){selected.value=[...new Set([...selected.value,...visible.value.filter(m=>!existing.value.has(m.type)).map(m=>m.type)])]}
</script>

<template>
  <n-modal :show="true" preset="card" :title="title" class="message-picker" style="width:720px;max-width:95vw" :bordered="false" :closable="!loading" :mask-closable="!loading" :close-on-esc="!loading" @update:show="show=>{if(!show)emit('cancel')}">
    <p class="picker-description">{{source}} · 共 {{messages.length}} 种消息。勾选需要的接口，左侧仅创建选中模板，字典完整保留用于字段校验和响应解析。</p>
    <n-input v-model:value="query" placeholder="搜索接口名称或消息类型" clearable :disabled="loading"/>
    <div class="picker-toolbar"><n-button size="small" :disabled="loading" @click="selectVisible">全选当前结果</n-button><n-button size="small" :disabled="loading||!selected.length" @click="selected=[]">清空选择</n-button><span>已选 {{selected.length}} 个接口<span v-if="existing.size"> · {{existing.size}} 种已有模板</span></span></div>
    <div class="picker-list">
      <n-checkbox-group v-model:value="selected" :disabled="loading">
        <div v-for="m in visible" :key="m.type" class="picker-row">
          <n-checkbox :value="m.type" :disabled="loading||existing.has(m.type)" :aria-disabled="loading||existing.has(m.type)" :aria-label="m.name+' ('+m.type+')'"><span>{{m.name}} <code>({{m.type}})</code></span></n-checkbox>
          <small>{{existing.has(m.type)?'已存在':m.category==='admin'?'会话消息':'业务消息'}}</small>
        </div>
      </n-checkbox-group>
      <n-empty v-if="!visible.length" size="small" description="没有匹配的消息接口"/>
    </div>
    <template #footer><div class="picker-footer"><span>确认后仅添加 {{selected.length}} 个接口</span><n-button :disabled="loading" @click="emit('cancel')">取消</n-button><n-button type="primary" :disabled="!selected.length" :loading="loading" @click="emit('confirm',[...selected])">{{confirmText}}</n-button></div></template>
  </n-modal>
</template>

<style scoped>
.picker-description{font-size:12px;color:#8b819d;line-height:1.8;margin-bottom:16px;overflow-wrap:anywhere}.picker-toolbar{display:flex;align-items:center;gap:9px;margin:14px 0}.picker-toolbar>span{margin-left:auto;font-size:12px;color:#8b819d}.picker-list{height:350px;overflow:auto;border:1px solid #e9e5f0;border-radius:7px}.picker-row{display:flex;align-items:center;gap:12px;padding:10px 14px;border-bottom:1px solid #f0edf5}.picker-row:last-child{border-bottom:0}.picker-row .n-checkbox{flex:1;min-width:0}.picker-row code{color:#9b81bf;font-size:11px}.picker-row small{font-size:10px;color:#aaa1b8;flex-shrink:0}.picker-list>.n-empty{padding:35px}.picker-footer{display:flex;align-items:center;gap:10px}.picker-footer>span{font-size:12px;color:#8b819d;margin-right:auto}
</style>
