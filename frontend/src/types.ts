export interface Service { id: string; name: string; protocol: 'tcp'|'fix'; host: string; port: number; timeoutMs: number; dictionaryId: string; framing: string; delimiter: string; frameLength: number; frameFunctionId: string; sender: string; target: string; heartbeat: number; autoLogon: boolean; resetOnLogon: boolean; username: string; password: string }
export interface Request { id: string; serviceId: string; name: string; data: string; encoding: string; preScript: string; postScript: string; requestFunctionId: string; responseFunctionId: string; messageType: string; values: Record<string, any>; rawMode: boolean }
export interface CustomFunction { id: string; name: string; kind: 'request'|'response'|'frame'; code: string }
export interface DictionaryFile { id: string; name: string; xml: string }
export interface Workspace { version: number; services: Service[]; requests: Request[]; functions: CustomFunction[]; dictionaries: DictionaryFile[] }
export interface SessionState { connected: boolean; loggedOn: boolean; nextSeq: number; expectedSeq: number; tasks: string[] }
export interface Event { id: number; time: string; serviceId: string; kind: string; direction?: string; text: string; raw?: string; hex?: string; base64?: string; size?: number }
export interface State { workspace: Workspace; sessions: Record<string, SessionState>; events: Event[] }
export interface FieldEnum { value: string; description: string }
export interface Field { number: number; name: string; type: string; enums?: FieldEnum[] }
export interface FixNode extends Field { kind: 'field'|'component'|'group'; required: boolean; children?: FixNode[] }
export interface FixMessage { name: string; type: string; category: string; nodes: FixNode[] }
export interface Dictionary { version: string; fields: Field[]; messages: FixMessage[]; header: FixNode[]; trailer: FixNode[]; componentCount: number }
