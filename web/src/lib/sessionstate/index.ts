// Adapted from OpenCode packages/client/src/solid/data.ts at
// c55ee2a8152603f04a409163bd3edf79c425fbd7. See third_party/opencode.

export type JSONValue = null | boolean | number | string | JSONValue[] | { [key: string]: JSONValue }
export type JSONObject = { [key: string]: any }

export type ProjectionState = {
	info: Record<string, JSONObject>
	family: Record<string, string[]>
	active: Record<string, 'idle' | 'running'>
	message: Record<string, JSONObject[]>
	pending: Record<string, JSONObject[]>
	permission: Record<string, JSONObject[]>
	form: Record<string, JSONObject[]>
}

export type Snapshot = { state: ProjectionState }

const emptyState = (): ProjectionState => ({
	info: {}, family: {}, active: {}, message: {}, pending: {}, permission: {}, form: {}
})
const clone = <T>(value: T): T => structuredClone(value)
const eventMessageID = (id: string) => id.replace(/^evt_/, 'msg_')

const toolProgressMetadata = (state: JSONObject, value: JSONObject): JSONObject => {
	if (value?.mode !== 'append') return clone(value)
	let existing = state.metadata?.transcript
	if (!Array.isArray(existing)) existing = clone(state.content?.find((part: JSONObject) => part.type === 'transcript')?.events ?? [])
	const metadata = clone(value)
	delete metadata.mode
	if (Array.isArray(value.transcript)) existing.push(...clone(value.transcript))
	metadata.transcript = existing
	return metadata
}

export class SessionProjection {
	private state: ProjectionState
	private messageIndex = new Map<string, Map<string, JSONObject>>()
	private latestText = new Map<string, JSONObject>()
	private openReasoning = new Map<string, JSONObject[]>()
	private tools = new Map<string, JSONObject>()

	constructor(state?: ProjectionState) { this.state = clone(state ?? emptyState()); for (const sid of Object.keys(this.state.message)) this.rebuildIndexes(sid) }

	static restore(saved: Snapshot): SessionProjection {
		const machine = new SessionProjection(saved.state)
		return machine
	}

	/** Reducer-owned live state for renderers. Callers must treat it as read-only. */
	viewState(): Readonly<ProjectionState> { return this.state }
	snapshot(): Snapshot { return clone({ state: this.state }) }

	apply(event: JSONObject) {
		const d = event.data ?? {}
		const sid = d.sessionID
		switch (event.type) {
			case 'session.deleted': this.removeSession(sid); break
			case 'session.usage.updated': if (this.state.info[sid]) Object.assign(this.state.info[sid], { cost: d.cost, tokens: clone(d.tokens) }); break
			case 'session.agent.selected': {
				const previous = this.state.info[sid]?.agent
				if (this.state.info[sid]) this.state.info[sid].agent = clone(d.agent)
				this.insert(sid, this.withDefined({ id: eventMessageID(event.id), type: 'agent-switched', agent: clone(d.agent), previous, time: { created: event.created } }))
				break
			}
			case 'session.model.selected':
				if (this.state.info[sid]) this.state.info[sid].model = clone(d.model)
				if (this.state.message[sid]) {
					const messageID = eventMessageID(event.id)
					this.insert(sid, { id: messageID, type: 'model-switched', model: clone(d.model), time: { created: event.created } })
				}
				break
			case 'session.permissions.updated': if (this.state.info[sid]) this.state.info[sid].permissions = clone(d.permissions); break
			case 'session.moved': this.move(event); break
			case 'session.inbox.enqueued': this.admit({ id: d.inboxID, sessionID: sid, timeCreated: event.created, ...clone(d.item) }); break
			case 'session.inbox.delivered': this.deliver(sid, d.inboxID, event.created); break
			case 'session.inbox.delivery.changed': this.updatePending(sid, d.inboxID, d.delivery); break
			case 'session.inbox.cancelled': this.retract(sid, d.inboxID); break
			case 'session.instructions.updated': if (d.text !== undefined) this.insert(sid, { id: eventMessageID(event.id), type: 'system', text: d.text, description: `Instructions updated: ${Object.keys(d.delta).join(', ')}`, ...(event.metadata === undefined ? {} : { metadata: clone(event.metadata) }), time: { created: event.created } }); break
			case 'session.synthetic': this.insert(sid, this.withDefined({ id: eventMessageID(event.id), type: 'synthetic', text: d.text, description: d.description, metadata: d.metadata, time: { created: event.created } })); break
			case 'session.shell.started': this.shellStarted(event); break
			case 'session.shell.ended': this.shellEnded(event); break
			case 'session.step.started': this.stepStarted(event); break
			case 'session.step.streamed': this.editAssistant(sid, d.assistantMessageID, a => { a.time.streamed = event.created }); break
			case 'session.step.ended': this.stepEnded(event); break
			case 'session.step.failed': this.stepFailed(event); break
			case 'session.text.started': this.editAssistant(sid, d.assistantMessageID, a => { const part={ type: 'text', text: '' };a.content.push(part);this.latestText.set(this.partKey(sid,d.assistantMessageID),part) }); break
			case 'session.text.delta': this.editText(sid, d.assistantMessageID, t => { t.text += d.delta }); break
			case 'session.text.ended': this.editText(sid, d.assistantMessageID, t => { t.text = d.text }); break
			case 'session.tool.input.started': this.editAssistant(sid, d.assistantMessageID, a => { const part={ type: 'tool', id: d.id, name: d.name, time: { created: event.created }, state: { status: 'streaming', input: '' } };a.content.push(part);this.tools.set(this.partKey(sid,d.assistantMessageID,d.id),part) }); break
			case 'session.tool.input.delta': this.editTool(sid, d.assistantMessageID, d.id, t => { if (t.state.status === 'streaming') t.state.input += d.delta }); break
			case 'session.tool.input.ended': this.editTool(sid, d.assistantMessageID, d.id, t => { if (t.state.status === 'streaming') t.state.input = d.text }); break
			case 'session.tool.called': this.toolCalled(event); break
			case 'session.tool.progress': this.editTool(sid, d.assistantMessageID, d.id, t => { if (['running', 'completed', 'error'].includes(t.state.status)) t.state.metadata = toolProgressMetadata(t.state, d.metadata) }); break
			case 'session.tool.success': this.toolSuccess(event); break
			case 'session.tool.failed': this.toolFailed(event); break
			case 'session.reasoning.started': this.editAssistant(sid, d.assistantMessageID, a => { const part=this.withDefined({ type: 'reasoning', text: '', state: clone(d.state), time: { created: event.created } });a.content.push(part);const key=this.partKey(sid,d.assistantMessageID),open=this.openReasoning.get(key)??[];open.push(part);this.openReasoning.set(key,open) }); break
			case 'session.reasoning.delta': this.editReasoning(sid, d.assistantMessageID, r => { r.text += d.delta }); break
			case 'session.reasoning.ended': this.reasoningEnded(event); break
			case 'session.retry.scheduled': this.editAssistant(sid, d.assistantMessageID, a => { a.retry = { attempt: d.attempt, at: d.at, error: clone(d.error) } }); break
			case 'session.execution.started': this.state.active[sid] = 'running'; break
			case 'session.execution.succeeded': case 'session.execution.failed': case 'session.execution.interrupted': this.executionEnded(event); break
			case 'session.revert.staged': if (this.state.info[sid]) this.state.info[sid].revert = clone(d.revert); break
			case 'session.revert.cleared': if (this.state.info[sid]) delete this.state.info[sid].revert; break
			case 'session.revert.committed': this.revertCommitted(sid, d.to); break
			case 'session.compaction.started': this.compactionStarted(event); break
			case 'session.compaction.delta': { const c = this.last(sid, x => x.type === 'compaction' && x.status === 'running'); if (c) c.summary += d.text; break }
			case 'session.compaction.ended': this.compactionEnded(event); break
			case 'session.compaction.failed': this.compactionFailed(event); break
			case 'permission.asked': { const list = this.state.permission[sid] ??= []; if (!list.some(x => x.id === d.id)) list.push(clone(d)); break }
			case 'permission.replied': { const request=this.state.permission[sid]?.find(x=>x.id===d.requestID);if(request){request.reply=d.reply;request.repliedAt=event.created}break }
			case 'form.replied': case 'form.cancelled': this.removeForm(sid, d.id, event.location); break
			case 'form.created': {
				if (!event.location) break
				const owner = d.form.sessionID, list = this.state.form[owner] ??= []
				if (!list.some(x => x.id === d.form.id)) list.push(clone(owner === 'global' ? { ...d.form, location: event.location } : d.form))
				break
			}
		}
	}

	private withDefined(value: JSONObject) { for (const key of Object.keys(value)) if (value[key] === undefined) delete value[key]; return value }
	private messages(sid: string) { return this.state.message[sid] ??= [] }
	private partKey(sid:string,messageID:string,suffix=''){return `${sid}\0${messageID}\0${suffix}`}
	private rebuildIndexes(sid:string){
		for(const map of [this.latestText,this.openReasoning,this.tools])for(const key of map.keys())if(key.startsWith(`${sid}\0`))map.delete(key)
		const index=new Map<string,JSONObject>();
		for(const item of this.state.message[sid]??[]){index.set(item.id,item);if(item.type!=='assistant')continue;for(const part of item.content??[]){const key=this.partKey(sid,item.id);if(part.type==='text')this.latestText.set(key,part);if(part.type==='reasoning'&&!part.time?.completed){const open=this.openReasoning.get(key)??[];open.push(part);this.openReasoning.set(key,open)}if(part.type==='tool')this.tools.set(this.partKey(sid,item.id,part.id),part)}}
		this.messageIndex.set(sid,index)
	}
	private insert(sid: string, item: JSONObject) { const list = this.messages(sid); let index=this.messageIndex.get(sid);if(!index){this.rebuildIndexes(sid);index=this.messageIndex.get(sid)!}if(index.has(item.id))return;const copied=clone(item);list.push(copied);index.set(copied.id,copied);if(copied.type==='assistant')this.rebuildIndexes(sid) }
	private upsertMessage(sid: string, item: JSONObject) { const list = this.messages(sid); const at = list.findIndex(x => x.id === item.id); if (at < 0) list.push(clone(item)); else list[at] = clone(item);this.rebuildIndexes(sid) }
	private last(sid: string, predicate: (x: JSONObject) => boolean) { return this.state.message[sid]?.findLast(predicate) }
	private editAssistant(sid: string, id: string, fn: (x: JSONObject) => void) { const x = this.messageIndex.get(sid)?.get(id); if (x?.type === 'assistant') fn(x) }
	private editText(sid: string, id: string, fn: (x: JSONObject) => void) { const x=this.latestText.get(this.partKey(sid,id));if(x)fn(x) }
	private editReasoning(sid: string, id: string, fn: (x: JSONObject) => void) { const key=this.partKey(sid,id),open=this.openReasoning.get(key),x=open?.at(-1);if(x){fn(x);if(x.time?.completed){open!.pop();if(!open!.length)this.openReasoning.delete(key)}} }
	private editTool(sid: string, id: string, toolID: string, fn: (x: JSONObject) => void) { const x=this.tools.get(this.partKey(sid,id,toolID));if(x)fn(x) }

	private removeSession(sid: string) {
		for (const key of ['info','active','message','pending','permission','form'] as const) delete this.state[key][sid]
		for (const root of Object.keys(this.state.family)) { this.state.family[root] = this.state.family[root].filter(x => x !== sid); if (!this.state.family[root].length) delete this.state.family[root] }
		this.messageIndex.delete(sid);for(const map of [this.latestText,this.openReasoning,this.tools])for(const key of map.keys())if(key.startsWith(`${sid}\0`))map.delete(key)
	}
	private admit(item: JSONObject) { const list = this.state.pending[item.sessionID] ??= []; const at = list.findIndex(x => x.id === item.id); if (at < 0) list.push(clone(item)); else list[at] = clone(item); if (item.type === 'user' || item.type === 'synthetic') this.upsertMessage(item.sessionID, { id: item.id, type: item.type, ...clone(item.payload), time: { created: item.timeCreated } }) }
	private retract(sid: string, id: string) { const pending=this.state.pending[sid];if(pending?.some(x=>x.id===id))this.state.pending[sid]=pending.filter(x=>x.id!==id);if(!this.messageIndex.get(sid)?.has(id))return;this.state.message[sid] = this.state.message[sid].filter(x => x.id !== id);this.rebuildIndexes(sid) }
	private deliver(sid: string, id: string, created: number) { const pending=this.state.pending[sid],admitted = pending?.some(x => x.id === id)??false;if(admitted)this.state.pending[sid]=pending!.filter(x=>x.id!==id); const list = this.state.message[sid];if(!list)return; const at = list.findIndex(x => x.id === id); if (admitted && at >= 0) { const [x] = list.splice(at,1); x.time.created = created; list.push(x);this.rebuildIndexes(sid) } }
	private updatePending(sid: string, id: string, delivery: JSONValue) { const x = this.state.pending[sid]?.find(x => x.id === id); if (x && x.delivery !== delivery) x.delivery = clone(delivery) }
	private move(event: JSONObject) { const d=event.data,s=this.state.info[d.sessionID]; if (!s) return; const previous=this.withDefined({location:clone(s.location),projectID:s.projectID,subpath:s.subpath}); s.location=clone(d.location); if(d.projectID) s.projectID=d.projectID; s.subpath=d.subpath; this.insert(d.sessionID,this.withDefined({id:eventMessageID(event.id),type:'location-switched',location:clone(d.location),projectID:d.projectID,subpath:d.subpath,previous,time:{created:event.created}})) }
	private shellStarted(event: JSONObject) { const d=event.data, sh=d.shell; const metadata=sh.metadata?.background===true?{...(event.metadata??{}),background:true}:event.metadata; this.insert(d.sessionID,this.withDefined({id:eventMessageID(event.id),type:'shell',shellID:sh.id,command:sh.command,status:sh.status,exit:sh.exit,metadata:clone(metadata),time:{created:event.created}})) }
	private shellEnded(event: JSONObject) { const d=event.data,x=this.last(d.sessionID,m=>m.type==='shell'&&m.shellID===d.shell.id); if(x){x.status=d.shell.status;x.exit=d.shell.exit;x.output=clone(d.output);x.time.completed=event.created} }
	private stepStarted(event: JSONObject) { const d=event.data; const existing=this.state.message[d.sessionID]?.find(x=>x.id===d.assistantMessageID); if(existing?.type==='assistant'){existing.agent=d.agent;existing.model=clone(d.model);for(const k of ['retry','error','finish','rawFinish','providerState'] as const)delete existing[k];delete existing.time.streamed;delete existing.time.completed;if(d.snapshot)existing.snapshot={...(existing.snapshot??{}),start:d.snapshot};return} const current=this.last(d.sessionID,x=>x.type==='assistant'&&!x.time.completed);if(current){delete current.retry;current.time.completed=event.created}this.insert(d.sessionID,this.withDefined({id:d.assistantMessageID,type:'assistant',agent:d.agent,model:clone(d.model),metadata:clone(event.metadata),content:[],snapshot:d.snapshot?{start:d.snapshot}:undefined,time:{created:event.created}})) }
	private stepEnded(event: JSONObject) { const d=event.data;this.editAssistant(d.sessionID,d.assistantMessageID,a=>{a.time.completed=event.created;a.finish=d.finish;a.rawFinish=d.rawFinish;a.providerState=clone(d.providerState);a.cost=d.cost;a.tokens=clone(d.tokens);if(d.snapshot)a.snapshot={...(a.snapshot??{}),end:d.snapshot};this.withDefined(a)}) }
	private stepFailed(event: JSONObject) { const d=event.data;this.editAssistant(d.sessionID,d.assistantMessageID,a=>{a.time.completed=event.created;a.finish=d.finish??'error';a.rawFinish=d.rawFinish;a.providerState=clone(d.providerState);a.error=clone(d.error);delete a.retry;if(d.cost!==undefined&&d.tokens!==undefined){a.cost=d.cost;a.tokens=clone(d.tokens)}this.withDefined(a)}) }
	private toolCalled(event: JSONObject) { const d=event.data;this.editTool(d.sessionID,d.assistantMessageID,d.id,t=>{t.time.ran=event.created;t.executed=d.executed;t.providerState=clone(d.state);t.state={status:'running',input:clone(d.input),metadata:{}};this.withDefined(t)}) }
	private toolSuccess(event: JSONObject) { const d=event.data;this.editTool(d.sessionID,d.assistantMessageID,d.id,t=>{if(t.state.status!=='running')return;const metadata=d.metadata===undefined?t.state.metadata:d.metadata;t.state=this.withDefined({status:'completed',input:clone(t.state.input),metadata:clone(metadata),content:clone(d.content)});t.executed=d.executed||t.executed===true;t.providerResultState=clone(d.resultState);t.time.completed=event.created;this.withDefined(t)}) }
	private toolFailed(event: JSONObject) { const d=event.data;this.editTool(d.sessionID,d.assistantMessageID,d.id,t=>{if(t.state.status!=='streaming'&&t.state.status!=='running')return;const metadata=d.metadata===undefined?t.state.metadata:d.metadata;t.state=this.withDefined({status:'error',error:clone(d.error),input:typeof t.state.input==='string'?{}:clone(t.state.input),metadata:clone(metadata),content:clone(d.content)});t.executed=d.executed||t.executed===true;t.providerResultState=clone(d.resultState);t.time.completed=event.created;this.withDefined(t)}) }
	private reasoningEnded(event: JSONObject) { const d=event.data;this.editReasoning(d.sessionID,d.assistantMessageID,r=>{r.text=d.text;r.time={created:r.time?.created??event.created,completed:event.created};if(d.state!==undefined)r.state=clone(d.state)}) }
	private executionEnded(event: JSONObject) { const d=event.data;this.state.active[d.sessionID]='idle';const a=this.last(d.sessionID,x=>x.type==='assistant'&&!x.time.completed);if(a)delete a.retry }
	private revertCommitted(sid:string,to:string){if(this.state.info[sid])delete this.state.info[sid].revert;this.state.pending[sid]=(this.state.pending[sid]??[]).filter(x=>x.id<to);const at=(this.state.message[sid]??[]).findIndex(x=>x.id>=to);if(at>=0){this.state.message[sid].splice(at);this.rebuildIndexes(sid)}}
	private compactionStarted(event:JSONObject){const d=event.data;if(d.inputID)this.state.pending[d.sessionID]=(this.state.pending[d.sessionID]??[]).filter(x=>x.id!==d.inputID);this.insert(d.sessionID,{id:d.inputID??eventMessageID(event.id),type:'compaction',status:'running',reason:d.reason,summary:'',recent:d.recent??'',time:{created:event.created}})}
	private compactionEnded(event:JSONObject){const d=event.data,c=this.last(d.sessionID,x=>x.type==='compaction'&&x.status==='running');const fields={status:'completed',reason:d.reason,model:clone(d.model),providerState:clone(d.providerState),providerContext:clone(d.providerContext),summary:d.text,recent:d.recent,cost:d.cost,tokens:clone(d.tokens)};if(c){Object.assign(c,this.withDefined(fields));return}this.insert(d.sessionID,this.withDefined({id:eventMessageID(event.id),type:'compaction',...fields,time:{created:event.created}}))}
	private compactionFailed(event:JSONObject){const d=event.data;if(d.inputID)this.state.pending[d.sessionID]=(this.state.pending[d.sessionID]??[]).filter(x=>x.id!==d.inputID);const list=this.messages(d.sessionID),at=list.findLastIndex(x=>x.type==='compaction'&&x.status==='running'),c=list[at];const failed=this.withDefined({id:c?.id??d.inputID??eventMessageID(event.id),type:'compaction',status:'failed',reason:d.reason??'manual',error:clone(d.error??{type:'compaction.failed',message:'Compaction failed before recording an error'}),metadata:clone(c?.type==='compaction'?c.metadata:event.metadata),cost:d.cost,tokens:clone(d.tokens),time:clone(c?.type==='compaction'?c.time:{created:event.created})});if(c?.type==='compaction')list[at]=failed;else this.insert(d.sessionID,failed)}
	private removeForm(sid:string,id:string,ref?:JSONObject){const forms=this.state.form[sid];if(!forms)return;this.state.form[sid]=forms.filter(f=>f.id!==id||(sid==='global'&&ref&&f.location&&JSON.stringify([f.location.directory,f.location.workspaceID])!==JSON.stringify([ref.directory,ref.workspaceID])))}
}
