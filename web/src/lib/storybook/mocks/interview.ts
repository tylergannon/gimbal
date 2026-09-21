import type { InterviewAnswer, InterviewAnswered } from "../../../routes/types.js";
import { mockForm } from "../remotes.js";

export type { InterviewAnswer, InterviewAnswered };

export const answerInterview = mockForm<InterviewAnswer, InterviewAnswered>("answerInterview", {
  accepted: true,
} as InterviewAnswered);
