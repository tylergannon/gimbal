import {readFileSync} from 'node:fs';
import { RunObservation } from '../../../web/src/lib/observation/index.ts';
const dir=import.meta.dir+'/probe-artifacts/';
const snapshot=JSON.parse(readFileSync(dir+'cancelled-snapshot.json','utf8'));
const observation=new RunObservation({run:{id:'probe',name:'cancelled',status:'running',sessions:{}},invocations:{}});
const connection=observation.beginConnection();
for (const line of readFileSync(dir+'cancelled-events.jsonl','utf8').trim().split('\n')) observation.apply({type:'lifecycle',data:JSON.parse(line)},connection);
console.log(JSON.stringify({liveBrowserStatus:observation.run.status,reloadedServerStatus:snapshot.run.status}));
