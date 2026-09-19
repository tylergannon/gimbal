import {
  SessionProjection,
  type JSONObject,
  type JSONValue,
  type ProjectionState,
  type Snapshot,
} from "../sessionstate/index.js";

/** The five token counts every harness reports, flat. A count the provider
 * did not report is 0: zero is a number, and nothing here says whether it was
 * measured or absent. */
export type Tokens = {
  input: number;
  cache_read: number;
  cache_write: number;
  output: number;
  reasoning: number;
};
/** Tokens and the cost the harness stated, 0 when it stated none. */
export type Usage = Tokens & { stated_cost: number };

export type RunStatus = "running" | "completed" | "failed" | "cancelled";
/** The rows of the run store, as Go writes them. Times are Unix ms. */
export type RunRow = {
  id: string;
  name: string;
  status: RunStatus;
  error: string;
  started: number;
  ended: number;
};
/** One planner decision: the record's sequence and the event's own words. */
export type Decision = { seq: number; body: JSONValue };
export type ValueArtifact = {
  file: string;
  size: number;
  format: "text" | "json";
  preview: string;
};
export type ScopeValue = { value?: JSONValue; artifact?: ValueArtifact };
/** One scope instance. `key` is the slash path, so the parent is the path
 * above it. An ended scope with no error is 'ended', never 'succeeded'.
 * `loop` marks a PromiseLoop's own scope, whose planner a person at the page can
 * send a message to while the run is in progress. */
export type ScopeRow = {
  run: string;
  key: string;
  name: string;
  loop: boolean;
  status: "running" | "ended";
  error: string;
  task?: JSONValue;
  began: number;
  ended: number;
  values: Record<string, ScopeValue>;
  decisions: Decision[];
};
/** One agent conversation. `scope` is where it was created, which is not
 * where its turns necessarily ran. */
export type SessionRow = {
  run: string;
  id: string;
  name: string;
  adapter: string;
  model: string;
  scope: string;
  parent: string;
  created: number;
};
/** One interview question. An accepted answer updates the same row. */
export type InterviewRow = {
  run: string;
  question_id: string;
  name: string;
  scope: string;
  session: string;
  question: string;
  status: "pending" | "answered";
  answer: string;
  asked: number;
  answered: number;
};
/** One agent turn. `scope` is where the turn ran, which is what its tokens
 * are charged to. */
export type TurnRow = {
  run: string;
  id: string;
  session: string;
  scope: string;
  prompt: string;
  output_type: string;
  result: string;
  error: string;
  interrupted: boolean;
  started: number;
  ended: number;
  duration: number;
};
/** One command run by the workflow, matching the public command table. */
export type CommandRow = {
  run: string;
  id: string;
  scope: string;
  name: string;
  command: string;
  args: string[];
  workdir: string;
  exit_code: number;
  stdout: string;
  stderr: string;
  stdout_file: string;
  stderr_file: string;
  error: string;
  interrupted: boolean;
  started: number;
  ended: number;
  duration: number;
};
/** One step that reached a model: the drill below a turn. It is a fact and is
 * never summed. */
export type ModelCallRow = Tokens & {
  run: string;
  turn: string;
  message: string;
  model: string;
  started: number;
  ended: number;
};
/** One roll-up: across every model, and per model. */
export type Total = { all: Usage; by_model: Record<string, Usage> };
/** Every scope's and every session's roll-up, computed in Go. The browser
 * never sums anything. */
export type Totals = { scopes: Record<string, Total>; sessions: Record<string, Total> };
/** One turn's transcript: the session projection and its native provenance. */
export type Transcript = { snapshot: Snapshot; provenance: Record<string, unknown> };

export type RunSnapshot = {
  stream: string;
  position: number;
  run: RunRow;
  scopes: Record<string, ScopeRow>;
  sessions: Record<string, SessionRow>;
  interviews: Record<string, InterviewRow>;
  turns: Record<string, TurnRow>;
  turn_usage: Record<string, Record<string, Usage>>;
  model_calls: Record<string, ModelCallRow[]>;
  commands: Record<string, CommandRow>;
  totals: Totals;
  transcripts: Record<string, Transcript>;
};

/** One changed row. The table name is the snapshot's own key, so a frame
 * names one thing: for turn_usage the row is the turn's whole model map, and
 * for model_calls the turn's whole call list. */
export type RowFrame =
  | { table: "run"; key: string; row: RunRow }
  | { table: "scopes"; key: string; row: ScopeRow }
  | { table: "sessions"; key: string; row: SessionRow }
  | { table: "interviews"; key: string; row: InterviewRow }
  | { table: "turns"; key: string; row: TurnRow }
  | { table: "turn_usage"; key: string; row: Record<string, Usage> }
  | { table: "model_calls"; key: string; row: ModelCallRow[] }
  | { table: "commands"; key: string; row: CommandRow };

export type ObservationFrame =
  | { type: "snapshot"; data: RunSnapshot }
  | { type: "row"; data: RowFrame }
  | { type: "totals"; data: Totals }
  | {
      type: "event";
      data: {
        scope: string;
        session: string;
        turn: string;
        event: JSONObject;
        nativeRef?: JSONValue;
      };
    };

export type ObservationDelta = { stream: string; position: number; frames: ObservationFrame[] };

/** One turn's live transcript. */
export type Transcribed = { projection: SessionProjection; provenance: Record<string, unknown> };

const emptySnapshot = (): Snapshot => ({
  state: { info: {}, family: {}, active: {}, message: {}, pending: {}, permission: {}, form: {} },
});
const clone = <T>(value: T): T => structuredClone(value);

/** Live run state. Revision invalidates renderers without cloning transcript data per frame. */
export class RunObservation {
  run: RunRow;
  scopes: Record<string, ScopeRow> = {};
  sessions: Record<string, SessionRow> = {};
  interviews: Record<string, InterviewRow> = {};
  turns: Record<string, TurnRow> = {};
  turnUsage: Record<string, Record<string, Usage>> = {};
  modelCalls: Record<string, ModelCallRow[]> = {};
  commands: Record<string, CommandRow> = {};
  totals: Totals = { scopes: {}, sessions: {} };
  readonly transcripts = new Map<string, Transcribed>();
  revision = 0;
  private messageRevisions = new Map<string, number>();
  private snapshotRevision = 0;
  private connectionGeneration = 0;
  stream: string;
  position: number;

  constructor(snapshot: RunSnapshot) {
    this.run = clone(snapshot.run);
    this.stream = snapshot.stream;
    this.position = snapshot.position;
    this.replace(snapshot);
  }
  beginConnection(): number {
    return ++this.connectionGeneration;
  }
  endConnection(generation: number) {
    if (generation === this.connectionGeneration) this.connectionGeneration++;
  }
  isCurrentConnection(generation: number) {
    return generation === this.connectionGeneration;
  }

  replace(snapshot: RunSnapshot, generation?: number): boolean {
    if (generation !== undefined && !this.isCurrentConnection(generation)) return false;
    this.run = clone(snapshot.run);
    this.stream = snapshot.stream;
    this.position = snapshot.position;
    this.scopes = clone(snapshot.scopes ?? {});
    this.sessions = clone(snapshot.sessions ?? {});
    this.interviews = clone(snapshot.interviews ?? {});
    this.turns = clone(snapshot.turns ?? {});
    this.turnUsage = clone(snapshot.turn_usage ?? {});
    this.modelCalls = clone(snapshot.model_calls ?? {});
    this.commands = clone(snapshot.commands ?? {});
    this.totals = clone(snapshot.totals ?? { scopes: {}, sessions: {} });
    this.messageRevisions.clear();
    this.snapshotRevision = this.revision + 1;
    this.transcripts.clear();
    for (const [turn, value] of Object.entries(snapshot.transcripts ?? {}))
      this.transcripts.set(turn, {
        projection: SessionProjection.restore(value.snapshot),
        provenance: clone(value.provenance),
      });
    this.revision++;
    return true;
  }

  applyDelta(delta: ObservationDelta, generation: number): boolean {
    if (
      !this.isCurrentConnection(generation) ||
      delta.stream !== this.stream ||
      delta.position !== this.position + 1
    )
      return false;
    for (const frame of delta.frames) this.apply(frame, generation);
    this.position = delta.position;
    return true;
  }

  apply(frame: ObservationFrame, generation: number): boolean {
    if (!this.isCurrentConnection(generation)) return false;
    switch (frame.type) {
      case "snapshot":
        return this.replace(frame.data, generation);
      case "row":
        this.setRow(frame.data);
        break;
      case "totals":
        this.totals = clone(frame.data);
        break;
      case "event": {
        const value = frame.data;
        const messageID = canonicalMessageID(value.event);
        if (messageID) this.messageRevisions.set(`${value.turn}\0${messageID}`, this.revision + 1);
        let transcript = this.transcripts.get(value.turn);
        if (!transcript) {
          transcript = { projection: SessionProjection.restore(emptySnapshot()), provenance: {} };
          this.transcripts.set(value.turn, transcript);
        }
        transcript.projection.apply(value.event);
        foldProvenance(transcript.provenance, value.event, value.nativeRef);
        break;
      }
    }
    this.revision++;
    return true;
  }

  /** One changed row replaces what was there. The run is one row and has no
   * key; every other table is keyed by the row's own id. */
  private setRow(frame: RowFrame) {
    switch (frame.table) {
      case "run":
        this.run = clone(frame.row);
        break;
      case "scopes":
        this.scopes[frame.key] = clone(frame.row);
        break;
      case "sessions":
        this.sessions[frame.key] = clone(frame.row);
        break;
      case "interviews":
        this.interviews[frame.key] = clone(frame.row);
        break;
      case "turns":
        this.turns[frame.key] = clone(frame.row);
        break;
      case "turn_usage":
        this.turnUsage[frame.key] = clone(frame.row);
        break;
      case "model_calls":
        this.modelCalls[frame.key] = clone(frame.row);
        break;
      case "commands":
        this.commands[frame.key] = clone(frame.row);
        break;
    }
  }

  state(turn: string): Readonly<ProjectionState> | undefined {
    return this.transcripts.get(turn)?.projection.viewState();
  }
  messageRevision(turn: string, message: string): number {
    return this.messageRevisions.get(`${turn}\0${message}`) ?? this.snapshotRevision;
  }
  snapshot(): RunSnapshot {
    return {
      stream: this.stream,
      position: this.position,
      run: clone(this.run),
      scopes: clone(this.scopes),
      sessions: clone(this.sessions),
      interviews: clone(this.interviews),
      turns: clone(this.turns),
      turn_usage: clone(this.turnUsage),
      model_calls: clone(this.modelCalls),
      commands: clone(this.commands),
      totals: clone(this.totals),
      transcripts: Object.fromEntries(
        [...this.transcripts].map(([turn, value]) => [
          turn,
          {
            snapshot: value.projection.snapshot(),
            provenance: clone(value.provenance),
          },
        ]),
      ),
    };
  }
}

export function canonicalMessageID(event: JSONObject): string | undefined {
  const data = event.data ?? {};
  if (typeof data.assistantMessageID === "string") return data.assistantMessageID;
  if (typeof data.messageID === "string") return data.messageID;
  if (typeof data.message?.id === "string") return data.message.id;
  if (typeof data.inboxID === "string") return data.inboxID;
  return undefined;
}

export function foldProvenance(
  target: Record<string, unknown>,
  event: JSONObject,
  nativeRef?: JSONValue,
) {
  if (nativeRef === undefined) return;
  const normalized =
    typeof nativeRef === "object" && nativeRef !== null && !Array.isArray(nativeRef)
      ? nativeRef.normalizedMessageID
      : undefined;
  const messageID = typeof normalized === "string" ? normalized : canonicalMessageID(event);
  if (messageID) target[messageID] = clone(nativeRef);
}

/** One message's own usage, flat and zero-filled. A message carries the
 * counts nested, the way the native event does; a roll-up is already flat and
 * comes from Go. */
export function usageOf(value: JSONObject | undefined): Usage {
  const tokens = (value?.tokens ?? {}) as JSONObject;
  const cache = (tokens.cache ?? {}) as JSONObject;
  return {
    input: numberOf(tokens.input),
    cache_read: numberOf(cache.read),
    cache_write: numberOf(cache.write),
    output: numberOf(tokens.output),
    reasoning: numberOf(tokens.reasoning),
    stated_cost: numberOf(value?.cost),
  };
}

/** One line of usage, for a message, a session total or a scope. */
export const usageText = (usage: Usage): string =>
  `${usage.input} in · ${usage.output} out · ${usage.reasoning} reasoning · ` +
  `${usage.cache_read} cache read · ${usage.cache_write} cache write · $${usage.stated_cost}`;

const numberOf = (value: unknown): number =>
  typeof value === "number" && Number.isFinite(value) ? value : 0;
