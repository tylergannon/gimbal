import type { Conversation, RequestState, Run } from '#lib/skgo/conversation/types';
import type { JSONObject } from '#lib/sessionstate';
export type SessionTurn = { request: RequestState; ordinal: number; messages: JSONObject[] };
export function sessionTurns(record: Conversation, messages: JSONObject[]) {
  const requests = record.requests.filter(request => request.kind === 'prompt' && request.delivery === 'turn');
  const used = new Set<JSONObject>();
  const turns: SessionTurn[] = requests.map((request, index) => ({ request, ordinal:index+1, messages:messages.filter(message => {
    const source = message.type === 'user' ? record.requests.find(item => item.id === message.requestID) : undefined;
    const belongs = String(message.id).startsWith(`${request.id}/`) || message.requestID === request.id || (source?.delivery === 'steer' && source.target === request.id);
    if (belongs) used.add(message); return belongs;
  }) }));
  return { turns, history:messages.filter(message => !used.has(message)) };
}
/** Only completed launch output and explicit request identity establish a source turn. */
export function launchedRuns(part: JSONObject, record: Pick<Conversation,'id'|'runs'>): Run[] {
  const command = part.state?.input?.command;
  if (part.type !== 'tool' || !['Bash','commandExecution'].includes(part.name) || part.state?.status !== 'completed' || typeof command !== 'string') return [];
  const args = command.split(/\s+/).map(value => value.replace(/^['"]|['"]$/g,''));
  const flag = args.indexOf('--conversation');
  const source = flag >= 0 ? args[flag+1] : args.find(value => value.startsWith('--conversation='))?.slice(15);
  const runIndex = args.indexOf('run');
  if (source !== record.id || runIndex < 0) return [];
  const lines = (part.state.content ?? []).filter((item: JSONObject) => item.type === 'text').flatMap((item: JSONObject) => String(item.text ?? '').split('\n').map(line => line.trim()));
  return record.runs.filter(run => args[runIndex+1] === run.workflow && lines.includes(run.id));
}
export function runEvents(message: JSONObject, record: Pick<Conversation,'id'|'runs'>, request?:RequestState): Record<number,Run[]> {
  if (!request || !String(message.id).startsWith(`${request.id}/`)) return {};
  return Object.fromEntries((message.content ?? []).map((part:JSONObject,index:number) => [index,launchedRuns(part,record)]));
}

/** Execution begins at the recorded user delivery, not when an idle draft was opened. */
export function sessionTurnSeconds(turn: SessionTurn, now?: number): number | undefined {
  const start = turn.messages.find(message => message.type === 'user' && message.requestID === turn.request.id)?.time?.created;
  if (typeof start !== 'number' || start <= 0) return undefined;
  const completed = turn.messages.map(message => message.time?.completed).filter(value => typeof value === 'number');
  const end = now ?? (completed.length ? Math.max(...completed) : undefined);
  return typeof end === 'number' ? Math.max(0,Math.floor((end-start)/1000)) : undefined;
}
