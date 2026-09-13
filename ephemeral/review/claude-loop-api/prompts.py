#!/usr/bin/env python3
"""Extract what each agent was actually shown: turn_started prompts by scope and session, headings only."""
import json, sys, re
path = sys.argv[1]
for line in open(path):
    try: r = json.loads(line)
    except: continue
    e = r.get("event", {})
    k = e.get("kind")
    if k == "turn_started":
        prompt = e.get("prompt", "")
        heads = re.findall(r"^## (.+)$", prompt, re.M)
        roles = re.findall(r"^## role\n\n(.+)$", prompt, re.M)
        print(f"[{r['seq']:>3}] {r['scope'] or '<root>'} :: {r.get('session')} :: {len(prompt)} bytes :: sections={heads} :: role={roles}")
    elif k == "planner_decision":
        t = e.get("task")
        print(f"[{r['seq']:>3}] {r['scope']} :: planner_decision :: {t['name'] if t else 'END DISPATCH'}")
    elif k == "value_set":
        print(f"[{r['seq']:>3}] {r['scope'] or '<root>'} :: value_set {e['key']!r} ({len(e['value'])} bytes)")
    elif k in ("run_ended", "run_cancelled", "complete"):
        print(f"[{r['seq']:>3}] {k}: {e}")
