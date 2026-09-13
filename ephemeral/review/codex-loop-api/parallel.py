import json,subprocess,sys
from pathlib import Path
w=Path(sys.argv[1]); b=w/'mergequeue'
p=subprocess.run(['go','build','-o',str(b),'.'],cwd=w,capture_output=True,text=True)
if p.returncode: print(p.stderr);sys.exit(1)
prs=[dict(id=i,deps=[],checks='pass') for i in [4,3,2,1]]+[dict(id=5,deps=[1,2],checks='pass')]
cases=[(['--parallel','2'],0,[[1,2],[3,4],[5]]),(['--parallel','1'],0,[[1],[2],[3],[4],[5]]),(['--parallel','0'],2,None),(['--parallel','-1'],2,None),(['--parallel','cat'],2,None),(['--parallel'],2,None)]
failed=[]
for args,code,waves in cases:
 p=subprocess.run([str(b)]+args,input=json.dumps(dict(prs=prs)),capture_output=True,text=True,timeout=5)
 ok=p.returncode==code
 if code==0:
  try:ok=ok and json.loads(p.stdout)==dict(waves=waves,blocked=[])
  except ValueError:ok=False
 else:ok=ok and bool(p.stderr.strip()) and not p.stdout.strip()
 if not ok:failed.append(args);print('FAIL',args,'exit',p.returncode,'stdout',p.stdout.strip(),'stderr',p.stderr.strip())
print(f'{len(cases)-len(failed)}/{len(cases)} parallelism cases passed')
sys.exit(bool(failed))
