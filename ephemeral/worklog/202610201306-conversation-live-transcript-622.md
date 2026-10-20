# gimbal#622 conversation live transcript projection

decision: Conversation.State is rebuilt in Manager.Get from the run journal via registry.Snapshot (turn transcripts), not from a new reducer or transport. Source: #619 moved chats to Interactive and dropped the only writer of State (old Manager.activity), leaving projections/State dead.
decision: turn->request pairing is by the person's text after "Human input:" in TurnRow.Prompt, in admission order; steers pair by inboxID turn/steer/<request id>. No journal schema change.
friction: Interactive publishes the user message into InteractiveTranscript only after the turn ends, and Get overwrites item.Messages from it, so Messages never holds the in-flight prompt. Left alone (Messages is only the UI fallback).
doc_bug: #619 also removed fileJournal (attached-file raw reads hidden from the readout); attached-file tool output now reaches State unredacted. Not in scope of #622; needs its own issue.
friction: List() calls Get() per conversation, so every inventory refresh now also takes a registry snapshot and re-projects; fine at current sizes, revisit if tabs get slow.
decision: (second pass) turn->request pairing now uses the interactive record's ordered Consumed inputs (a prompt starts a turn when consumed; stopped-while-queued prompts are never consumed) and steer evidence from the turn/steer/<request> inbox ids; text is only a guard on the next consumed prompt. A turn with no consumed-input evidence stays unpaired rather than guessed.
decision: the projection also settles request status for what it paired (ended turn -> completed, steer -> running/completed, open turn -> active_request), so the existing Queued line in ConversationView needs no UI change.
decision: Claude replies exist only as the StructuredOutput tool input (completed value.decision.message or waiting message); the projection adds that as a text part when the step has no other text, since the page hides StructuredOutput.
friction: the global `vp` runs the global vite-plus and fails with vite_ssr_environment_not_runnable in web/; use web/node_modules/.bin/vp after `vp install --frozen-lockfile`.
doc_bug: AGENTS.md says `vp run test` is the web gate, but from a fresh worktree it hits the global vite-plus; the local binary is needed.
