# Route: UI inspiration and graph/run views

Use this route for overview/focus, program versus run, timeline hierarchy, loops, supervisors/commands, or prior-art viewer behavior. The UI reports are evidence about inspected products and proposals, not Gimble implementation.

- Overview plus focused detail, four drilldown levels, selection drawer: `ui-timelines-report.md:11-32`; leaf summary `ui-timelines-index-leaf.md:21-32`.
- Program projection versus observed run projection: `ui-timelines-report.md:34-61`.
- Timeline hierarchy, stable rows, overlap, pending/terminal states: `ui-timelines-report.md:63-95`.
- Loop/repeated occurrences and reversible collapsed groups: `ui-timelines-report.md:96-117`.
- Supervisor overlays and command boundaries: `ui-timelines-report.md:45-61,136-153`; Gimble command limits `recommendation.md:89-97`.
- Tenacious implementation versus absent proposals: `ui-tenacious-report.md:3-29,31-52,54-56`; leaf summary `ui-tenacious-index-leaf.md:1-13`.
- Extractor obligations and honest unresolved coverage: `ui-timelines-report.md:34-61,136-153`; `ui-graph-extraction-notes.md:13-34`.
- Workflow graph UI patterns (Airflow, Dagster, Argo, n8n): `ui-workflows-report.md:7-27,29-46`; compact leaf routing `ui-workflows-index-leaf.md:9-27`.
- Parent design direction and extraction vocabulary: `ui-design-brief.md:5-21,62-80`; `ui-graph-extraction-notes.md:9-34`.
- Interactive study checks and explicit limits: `ui-study-validation.md:3-16`.
- Graph-extraction task: GitHub #201; `issue-graph-extraction-body.md` and captured `issue-201.json`. Separate from Beta linter #162 and runtime usage view #173.
- Beta fallback using runtime-only scope/turn evidence: `ui-timelines-report.md:119-147`; current milestone authority `milestone-assessment.md:3-18` and concrete slices `delivery-slices.md:17-21`.

Transfer the interaction patterns (stable selection, reversible collapse, compact run cards, defensive transcript detail) while retaining Gimble's typed relations and evidence-backed runtime states. Do not infer implementation from prior art or parent design notes: these are inspiration and proposals, not shipped UI behavior.
