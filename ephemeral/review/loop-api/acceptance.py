import json,subprocess,sys,random
from pathlib import Path
w=Path(sys.argv[1]); b=w/'mergequeue'
build=subprocess.run(['go','build','-o',str(b),'.'],cwd=w,capture_output=True,text=True)
if build.returncode: print(build.stderr); sys.exit(1)
cases=[]
def pr(i,deps=[],checks='pass'):return dict(id=i,deps=deps,checks=checks)
def good(name,prs,waves,blocked): cases.append((name,json.dumps(dict(prs=prs)),0,dict(waves=waves,blocked=blocked)))
def bad(name,raw):cases.append((name,raw,2,None))
good('empty',[],[],[])
good('diamond',[pr(4,[2,3]),pr(3,[1]),pr(2,[1]),pr(1)],[[1],[2,3],[4]],[])
good('blocked transitively',[pr(1,checks='fail'),pr(2,[1]),pr(3,[2]),pr(4),pr(5,checks='pending'),pr(6,[5])],[[4]],[1,2,3,5,6])
good('all blocked',[pr(1,checks='pending'),pr(2,[1])],[],[1,2])
for name,prs in [('duplicate',[pr(1),pr(1)]),('missing',[pr(2,[1])]),('cycle',[pr(1,[2]),pr(2,[1])]),('blocked cycle',[pr(1,[2],'fail'),pr(2,[1],'pending')]),('zero',[pr(0)]),('negative',[pr(-2)]),('bad status',[pr(1,checks='green')])]:bad(name,json.dumps(dict(prs=prs)))
bad('malformed','{');bad('trailing','{"prs":[]} {"prs":[]}')
for seed in range(12):
 r=random.Random(seed); prs=[pr(i,[j for j in range(1,i) if r.random()<.2],r.choice(['pass','pass','pass','pending','fail'])) for i in range(1,15)]; merged=set(); waves=[]
 while True:
  wave=sorted(x['id'] for x in prs if x['id'] not in merged and x['checks']=='pass' and set(x['deps'])<=merged)
  if not wave:break
  waves.append(wave);merged.update(wave)
 blocked=sorted(x['id'] for x in prs if x['id'] not in merged);r.shuffle(prs);good('random DAG '+str(seed),prs,waves,blocked)
failed=[]
for name,raw,code,want in cases:
 p=subprocess.run([str(b)],input=raw,capture_output=True,text=True,timeout=5)
 ok=p.returncode==code
 if code==0:
  try:ok=ok and json.loads(p.stdout)==want
  except ValueError:ok=False
 else:ok=ok and bool(p.stderr.strip()) and not p.stdout.strip()
 if not ok:failed.append(name); print('FAIL',name,'exit',p.returncode,'stdout',p.stdout.strip(),'stderr',p.stderr.strip(),'expected',want)
print(f'{len(cases)-len(failed)}/{len(cases)} acceptance cases passed')
sys.exit(bool(failed))
