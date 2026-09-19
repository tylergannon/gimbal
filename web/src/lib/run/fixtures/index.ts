/** The design's four scenarios, typed with the app's own types: a `Graph`
 * from `lib/workflow/types.ts` and the observation rows of
 * `lib/observation/index.ts`. Every fixture carries the text its specimen
 * carries, so a story reads the way the drawing reads. Nothing here fetches
 * anything; a component is handed one of these and nothing else. */

export { at, noTokens, total, usage } from "./support.js";
export {
  implementInterviewGraph,
  implementInterviewRun,
  implementInterviewSnapshot,
} from "./implement-interview.js";
export { planTripGraph, planTripPending, planTripRun, planTripSnapshot } from "./plan-trip.js";
export { recordedFailure, recordedRun, recordedRunSnapshot } from "./recorded-run.js";
export {
  runsList,
  runsListAttention,
  runsListCommands,
  runsListInterviews,
  runsListRuns,
  runsListScopes,
  runsListTurns,
} from "./runs-list.js";
