# Workflow UI research follow-up

- Request: research varied workflow/trace UI approaches with smaller agents, download Tenacious, propose a useful Beta viewer, and update graph-extraction task notes.
- Delegated three independent research lanes to gpt-5.6-luna: workflow products, timeline/trace products, and pinned Tenacious source. Reused the semantic-index agent after all reports and the parent synthesis existed.
- Result: local design brief and ready-to-apply extraction issue addition in `ephemeral/research/graphs`; all source evidence is cached there as flat files. Tenacious archive is pinned to d3a4d7ff9f5445a42a7516eda845f8c58a531a45.
- Decision: default map, linked timeline, persistent selection/inspector, optional hierarchy navigation. Distinguish scope containment, control flow, session ownership, and supervision; show commands as real operation kinds without pretending source recognition is process telemetry.
- Beta boundary: useful observed structure can ship before full static extraction; bounded Set linting remains in Beta and dynamic keys remain legal.
- Target ambiguity: live issue inventory did not reveal a dedicated graph-extraction issue. Asked user for its number or confirmation they intended a new issue. Prepared full addition locally; did not modify unrelated #162/#173 to pretend completion.
- Prototype: checked historical failure with ongoing run, linked map/timeline selection, Program mode, transcript drilling, and narrow viewport. Illustrative data only. Validation details are in `ui-study-validation.md`.
- Tool friction: render helper correctly refused overwriting an existing temporary preview until `--force` was supplied. Semantic-index first pass ran before one research lane had landed; final pass removed the stale absence debt.
- Follow-up: user delegated the new-versus-existing issue choice. Rechecked live issues and opened #201 for source graph extraction, with bounded acceptance and pinned research links. Kept it outside Beta and cross-referenced #162/#173 in its body. Updated the local extraction notes and index; captured the created issue locally.
