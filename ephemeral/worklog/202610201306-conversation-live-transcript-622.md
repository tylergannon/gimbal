# gimbal#622 conversation live transcript projection

decision: Conversation.State is rebuilt in Manager.Get from the run journal via registry.Snapshot (turn transcripts), not from a new reducer or transport. Source: #619 moved chats to Interactive and dropped the only writer of State (old Manager.activity), leaving projections/State dead.
decision: turn->request pairing is by the person's text after "Human input:" in TurnRow.Prompt, in admission order; steers pair by inboxID turn/steer/<request id>. No journal schema change.
friction: Interactive publishes the user message into InteractiveTranscript only after the turn ends, and Get overwrites item.Messages from it, so Messages never holds the in-flight prompt. Left alone (Messages is only the UI fallback).
doc_bug: #619 also removed fileJournal (attached-file raw reads hidden from the readout); attached-file tool output now reaches State unredacted. Not in scope of #622; needs its own issue.
friction: List() calls Get() per conversation, so every inventory refresh now also takes a registry snapshot and re-projects; fine at current sizes, revisit if tabs get slow.
