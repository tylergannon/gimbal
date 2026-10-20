import type {
  InteractiveAnswer,
  InteractiveAnswered,
  InteractiveFinish,
} from "../../../routes/types.js";
import { mockForm, mockCommand } from "../remotes.js";

export type { InteractiveAnswer, InteractiveAnswered, InteractiveFinish };

export const answerInteractive = mockForm<InteractiveAnswer, InteractiveAnswered>(
  "answerInteractive",
  {
    accepted: true,
  } as InteractiveAnswered,
);

export const finishInteractive = mockCommand<InteractiveFinish, InteractiveAnswered>(
  "finishInteractive",
  { accepted: true },
);
