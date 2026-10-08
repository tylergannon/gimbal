import assert from 'node:assert/strict';
import { readFile, mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';
import test from 'node:test';
import { layoutWorkflow } from './layout.ts';

// Pass gimbalgen's actual fixture projection instead of maintaining a second
// handwritten graph that can drift from the Go source/extractor contract.
const temp = await mkdtemp(join(tmpdir(), 'gimbal-layout-'));
const fixture = process.env.FLOWVIEW_FIXTURE || join(temp, 'routing.json');
let page;
try {
  if (!process.env.FLOWVIEW_FIXTURE) execFileSync('go', ['run', '../gimbalgen', '-dir', '../testdata/inspection', '-entry', 'Routing', '-name', 'routing', '-o', fixture]);
  page = JSON.parse(await readFile(fixture, 'utf8'));
} finally { await rm(temp, {recursive:true}); }
function allNodes(body) {
  return body.flatMap(n => [n, ...(n.children || []).flatMap(allNodes)]);
}
function absoluteOrigins(nodes) {
  const origins = new Map();
  for (const n of nodes) {
    const parent = origins.get(n.parentId || '') || {x:0,y:0};
    origins.set(n.id, {x: parent.x+n.position.x, y: parent.y+n.position.y});
  }
  return origins;
}
function checkGeometry(nodes, edges) {
  const origins = absoluteOrigins(nodes);
  for (const e of edges) {
    assert(e.data?.sections.length, `${e.id}: missing route`);
    const first = e.data.sections[0], last = e.data.sections.at(-1);
    for (const [id, handle, point] of [
      [e.source,e.sourceHandle,first.startPoint], [e.target,e.targetHandle,last.endPoint],
    ]) {
      const n = nodes.find(n=>n.id===id);
      const port = n.data.ports?.find(p=>p.id===handle);
      assert(port, `${id}: missing handle ${handle}`);
      const origin = origins.get(id);
      assert(Math.abs(point.x-origin.x-port.x)<1 && Math.abs(point.y-origin.y-port.y)<1,
        `${e.id}: nested route fails to meet ${id}`);
    }
    for (const label of e.data.labels) {
      const center = {x:label.x+label.width/2,y:label.y+label.height/2};
      assert(e.data.sections.some(section=>{
        const points=[section.startPoint,...section.bendPoints||[],section.endPoint];
        return points.some((p,i)=>{
          if(!i)return false;
          const previous=points[i-1];
          return center.x>=Math.min(p.x,previous.x)-1 && center.x<=Math.max(p.x,previous.x)+1 &&
            center.y>=Math.min(p.y,previous.y)-1 && center.y<=Math.max(p.y,previous.y)+1;
        });
      }), `${e.id}: label does not occlude its own route`);
    }
  }
}

test('real extracted branching fixture: compounds, ordered else-if, inline edge labels', async ()=>{
  const source = allNodes(page.body);
  const expanded = new Set(source.flatMap(n=>[n.id, `${n.id}-writes`]));
  const result=await layoutWorkflow(page,expanded);
  checkGeometry(result.nodes,result.edges);
  for (const decision of result.nodes.filter(n=>n.data.kind==='Condition')) {
    const merge=result.nodes.find(n=>n.id===`${decision.id}-merge`);
    if (merge) assert(Math.abs(decision.position.x+decision.width/2-merge.position.x-merge.width/2)<1,
      `${decision.id}: continuation drifts away from the decision spine`);
  }
  for(const scope of result.nodes.filter(n=>n.data.kind==='container'))
    assert(scope.width >= Math.min(340, scope.data.label.length*8+96), `${scope.id}: container header is truncated`);
  assert(!result.edges.some(e=>e.id.includes('layout-spine') || e.sourceHandle?.endsWith('-spine')),
    'layout constraints must never appear as execution paths');
  const conditional=source.find(n=>n.control?.kind==='if' && (n.branches?.length||0)>=3);
  assert(conditional, 'fixture must include if/else-if/else');
  const next=result.nodes.find(n=>n.id===`${conditional.id}-decision-1`);
  assert(next, 'else-if must have its own ordered decision');
  assert(result.edges.some(e=>e.source===conditional.id && e.target===next.id && e.sourceHandle?.endsWith('-alternative')));
  assert(!result.edges.some(e=>e.source===conditional.id && e.target===conditional.children[1][0].id), 'else-if body cannot bypass its own test');
  assert(result.edges.some(e=>e.data?.labels.some(l=>l.text===conditional.branches?.[0].label)), 'authored affirmative caption must survive');
  assert(result.nodes.some(n=>n.parentId && n.data.kind==='container'), 'fixture must exercise nested scopes');
  const writes=result.nodes.filter(n=>n.data.kind==='writes');
  assert(writes.length>0 && writes.every(n=>n.data.expanded && n.height===44+32*n.data.members.length));
  assert(writes.every(n=>!result.nodes.some(child=>child.parentId===n.id)), 'writes should be compact rows in one graph node');
});

test('closing one source scope does not collapse unrelated expanded scopes', async ()=>{
  const source=allNodes(page.body);
  const expanded=new Set(source.map(n=>n.id));
  const first=source.find(n=>n.kind==='Repeat');
  assert(first, 'fixture must include a loop');
  const initial=await layoutWorkflow(page,expanded);
  expanded.delete(first.id);
  const closed=await layoutWorkflow(page,expanded);
  const descendants=new Set(allNodes(first.children.flat()).map(n=>n.id));
  assert(!closed.nodes.some(n=>descendants.has(n.id)), 'closed scope descendants must disappear');
  assert(closed.nodes.find(n=>n.id===first.id)?.data.expanded===false);
  for(const n of initial.nodes.filter(n=>n.data.kind==='container' && n.id!==first.id && !descendants.has(n.id)))
    assert(closed.nodes.find(other=>other.id===n.id)?.data.expanded, `${n.id}: unrelated scope collapsed`);
  checkGeometry(closed.nodes,closed.edges);
});
