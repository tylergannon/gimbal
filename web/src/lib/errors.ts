import { isHttpError } from "@sveltejs/kit";

/** The message a person should read for a failed call; a server error carries it in its body. */
export function errorMessage(error: unknown): string {
  if (isHttpError(error)) return error.body.message;
  return error instanceof Error ? error.message : String(error);
}
