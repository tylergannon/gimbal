import type { CancelRun, ControlAccepted, StopTurn } from "../../../routes/types.js";
import { mockCommand } from "../remotes.js";

export type { CancelRun, ControlAccepted, StopTurn };

export const cancelRun = mockCommand<CancelRun, ControlAccepted>("cancelRun", { accepted: true });
export const stopTurn = mockCommand<StopTurn, ControlAccepted>("stopTurn", { accepted: true });
