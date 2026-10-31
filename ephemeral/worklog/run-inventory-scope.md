# Run inventory regression repair scope

Repair the four failing run inventory/publication tests on release candidate 4b828028. First observe the real production application. If inventory works, redesign obsolete raw-HTML assertions around meaningful current behavior; do not restore populated server HTML merely for the tests. Preserve real-server publication initialization, snapshots, running-to-completed event streams, actual hosted runs and reopening persistence. Observe home/runs inventory, run opening and live updates. Keep the repair bounded; release scheduling, hosting and qualification infrastructure are outside this task.

Current changes are tests only, plus this session's worklog. The generated watchRuns HTTP stream now carries the former inventory assertions. Browser coverage extends the existing production journey to empty home/runs, discovery, direct reloads, updates, transcript and graph navigation. Component coverage handles pending/completed and loading/empty transitions.

Local evidence available for independent inspection: /tmp/gimbal-inventory-proof/screenshots/inventory and /tmp/gimbal-inventory-proof/playwright-report/inventory. Baseline application production binary was built from 4b828028 with just build. No application source has changed.
