import type { LoopMessage, Sent, Steer, Waiting } from "../../../routes/types.js";
import { mockForm } from "../remotes.js";

export type { LoopMessage, Sent, Steer, Waiting };

export const steer = mockForm<Steer, Sent>("steer", { landed: true });
export const steerLoop = mockForm<LoopMessage, Waiting>("steerLoop", {
  message: "Waiting for the planner's next decision.",
});
