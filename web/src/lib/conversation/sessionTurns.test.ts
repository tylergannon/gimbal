import { expect,test } from 'vite-plus/test';
import type { Conversation } from '#lib/skgo/conversation/types';
import { sessionTurns,runEvents,sessionTurnSeconds } from './sessionTurns';

test('run provenance requires known completed launch output and explicit source request',()=>{
 const record = {id:'source',requests:[{id:'first',kind:'prompt',delivery:'turn'},{id:'second',kind:'prompt',delivery:'turn'}],runs:[{id:'real-run',workflow:'review',status:'completed'}]} as Conversation;
 const tool = {type:'tool',name:'Bash',state:{status:'completed',input:{command:'gimbal run review --conversation source'},content:[{type:'text',text:'real-run\n'}]}};
 const message = {id:'second/provider-id',type:'assistant',content:[tool]};
 const groups = sessionTurns(record,[{id:'user',type:'user',requestID:'second',text:'Review'},message]);
 expect(groups.turns[0].messages).toHaveLength(0);
 expect(groups.turns[1].messages).toHaveLength(2);
 expect(runEvents(message,record,groups.turns[1].request)[0]).toEqual(record.runs);
 expect(runEvents(message,record,groups.turns[0].request)).toEqual({});
 for(const changed of [ {...tool,state:{...tool.state,status:'running'}}, {...tool,state:{...tool.state,input:{command:'gimbal run review --conversation another'}}}, {...tool,state:{...tool.state,content:[{type:'text',text:'I launched real-run'}]}} ]) expect(runEvents({...message,content:[changed]},record,groups.turns[1].request)[0]).toEqual([]);
});


test('turn elapsed excludes idle draft time and remains unknown without recorded delivery',()=>{
 const turn = {request:{id:'request',created:1000},ordinal:1,messages:[{id:'user',type:'user',requestID:'request',time:{created:121000}},{id:'request/assistant',type:'assistant',time:{created:121100,completed:122000}}]} as Parameters<typeof sessionTurnSeconds>[0];
 expect(sessionTurnSeconds(turn)).toBe(1);
 expect(sessionTurnSeconds(turn,123000)).toBe(2);
 expect(sessionTurnSeconds({...turn,messages:turn.messages.slice(1)})).toBeUndefined();
});
