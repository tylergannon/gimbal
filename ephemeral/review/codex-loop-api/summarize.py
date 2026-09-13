from pathlib import Path
import json
base=Path(__file__).resolve().parent
runs=list((base/'project/runs').glob('*/run.jsonl'))
run=runs[0]
rows=[json.loads(l) for l in run.read_text().splitlines()]
summary={'head':'2a52971730d7e193b38e882e5b26b9885bd3edce','run':run.parent.name,'models':{},'tasks':[],'turns':[],'evidence':[],'terminal':None}
for r in rows:
 e=r['event']; k=e['kind']
 if k=='session_created': summary['models'][r['session']]=e['model']
 if k=='planner_decision':summary['tasks'].append(e.get('task'))
 if k=='turn_started':
  p=e['prompt']; role=p.split('## role\n\n')[1].split('\n\n')[0] if '## role\n\n' in p else None
  summary['turns'].append({'turn':r['turn'],'promptBytes':len(p.encode()),'role':role,'hasPreviousRecord':'Previous task record:' in p,'hasNewRequirement':'New requirement disclosed' in p})
 if k=='value_set' and e['key'] in ['acceptance evidence','repository checks','independent assessment','new requirement evidence','acceptance.py','parallel.py']:
  summary['evidence'].append({'scope':r['scope'],'key':e['key'],'value':json.loads(e['value'])})
 if k=='run_ended':summary['terminal']=e
(base/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
print('Run:',summary['run'],'terminal:',summary['terminal'])
for i,t in enumerate(summary['tasks']):print('Decision',i+1,':',t['name'] if t else 'STOP')
for e in summary['evidence']:
 v=e['value'];lines=[l for l in v.splitlines() if 'cases passed' in l or l.startswith('exit ') or l.startswith('PASS') or l.startswith('FAIL')]
 print(e['scope'],e['key'],lines)
