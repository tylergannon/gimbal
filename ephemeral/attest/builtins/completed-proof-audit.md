# Completed native proof audit

Audited the completed run records and native session transcripts for:

- `lfg-workspace/.gimble/runs/01M2HJDKYV9DBEVF5822RRMBNM.lfg`
- `plan-workspace/.gimble/runs/01M2HJDMNVNWA4AFSF35THCR5H.plan`
- `guided-workspace/.gimble/runs/01M2HJJ71JVCSA0Q3BHFT70AP2.work`

All three `run.json` records report `status: completed`, empty run errors, and a durable `complete` lifecycle event.

## LFG

The six table files and `run.jsonl` show one Codex worker session using `gpt-5.6-luna`, one Claude supervisor using `haiku`, one `supervise_attached` event with a 2 second interval, one worker turn, and seven supervisor turns (the final supervisor turn was interrupted during cleanup after the worker completed). The worker prompt names the absolute workspace, goal, acceptance, constraints, context file, and `python3 check.py`; the supervisor transcript shows it inspecting the brief and the worker's changes during the required delay and after file creation. The worker's native output reports the exact greeting and no remote or commit actions. The `check.1` scope records command `python3 check.py`, output `LFG_GREETING_OK`, exit code `0`. The independently rerun fixed check passed from the workspace.

## Guided route

The guided run has one Codex intake session (`gpt-5.6-luna`), one Codex LFG worker (`gpt-5.6-luna`), and one Claude supervisor (`haiku`). Its lifecycle has three turns: one recommendation, one worker turn, and one supervisor turn; `supervise_attached` is present at the LFG scope with the 30 second interval. The intake native output recommends `lfg` because the request is small and fully specified. The worker native output reports `status.txt` as exactly `ready\n`, and the `check.1` scope records `python3 check.py`, `GUIDED_LFG_OK`, exit code `0`. The independently rerun fixed check passed.

## Planning route

The plan run contains three independent draft sessions (Codex, Claude, Gemini), three independent cross-critique sessions, and one Gemini synthesizer session: seven turns total, all successful. Their native transcripts show the agents reading the local brief/check and producing a saved plan; the final artifact is `.gimble/requests/plan-2111880770/plan.md`, describing only `slug.py`, the exact algorithm, and the fixed 12-case check. The plan run's lifecycle ends at 03:42:48Z, while the current `slug.py` filesystem mtime is later (03:48:47Z); therefore the later implementation is not attributed to planning. The plan run's own native outputs repeatedly state that no files were changed or mutating commands run during planning. The later Sprint run and current filesystem are excluded from this planning claim.

## Fixed checks and input hashes

Commands were run with each workspace as the working directory:

- `lfg-workspace/check.py` -> `LFG_GREETING_OK`, exit `0`.
- `guided-workspace/check.py` -> `GUIDED_LFG_OK`, exit `0`.

SHA-256 hashes captured after the runs:

```text
bc29b15f7494c1ed1af3bb1734a540820863d2a62ff5dc6a62b1b52661ca0e8b  lfg-workspace/brief.txt
88a9be92801bf59cd1c8f3b1aa10c49e094c7ef7d700b545b999b244872bb038  lfg-workspace/check.py
8f31fe81e846ef7808b402c378e5fc03f4bae4c0a49b751f2c396cb761854ead  guided-workspace/check.py
b789956f275119fd44f4faba896e9848c002667283161b1d95d60d40d2d5df0b  plan-workspace/brief.md
a2eed5cd41a42c8240234cfa8c8177d0de1181408d085113f85383073d652f0e  plan-workspace/check.py
```

These hashes establish the observed input bytes for this audit; they do not infer historical state from the current repository status.
