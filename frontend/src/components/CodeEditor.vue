<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import * as monaco from 'monaco-editor/editor/editor.api.js'
import 'monaco-editor/languages/definitions/javascript/register.js'
import EditorWorker from 'monaco-editor/editor/editor.worker.js?worker'
;(self as any).MonacoEnvironment = { getWorker: () => new EditorWorker() }
const props=withDefaults(defineProps<{modelValue:string;language?:string;readonly?:boolean;compact?:boolean}>(),{language:'plaintext',readonly:false,compact:false})
const emit=defineEmits<{ 'update:modelValue':[value:string] }>()
const host=ref<HTMLElement>();let editor:monaco.editor.IStandaloneCodeEditor|undefined
onMounted(()=>{monaco.editor.defineTheme('tcp-light',{base:'vs',inherit:true,rules:[],colors:{'editor.background':'#ffffff','editorLineNumber.foreground':'#b1b4c7','editor.lineHighlightBackground':'#f8f7fd','editor.selectionBackground':'#e6ddff','editorCursor.foreground':'#7654e8','editorGutter.background':'#ffffff'}});editor=monaco.editor.create(host.value!,{value:props.modelValue||'',language:props.language,theme:'tcp-light',automaticLayout:true,editContext:false,minimap:{enabled:false},scrollbar:{alwaysConsumeMouseWheel:false},fontSize:12,fontFamily:'Consolas, "Cascadia Code", monospace',lineNumbers:'on',scrollBeyondLastLine:false,wordWrap:'on',readOnly:props.readonly,padding:{top:14,bottom:14},tabSize:2,overviewRulerLanes:0,renderLineHighlight:'line',contextmenu:false});editor.onDidChangeModelContent(()=>{const value=editor!.getValue();if(value!==props.modelValue)emit('update:modelValue',value)})})
watch(()=>props.modelValue,v=>{if(editor&&editor.getValue()!==(v||''))editor.setValue(v||'')});watch(()=>props.language,v=>{if(editor?.getModel())monaco.editor.setModelLanguage(editor.getModel()!,v)})
onBeforeUnmount(()=>{const model=editor?.getModel();editor?.dispose();model?.dispose()})
</script>
<template><div ref="host" class="code-editor" :class="{compact}"/></template>
