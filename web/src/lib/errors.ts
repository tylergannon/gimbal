import { isHttpError } from "@sveltejs/kit";

/** The message a person should read for a failed call; a server error carries it in its body. */
export function errorMessage(error: unknown): string {
  if (isHttpError(error)) return error.body.message;
  if (error instanceof Error) return error.message;
  // Live remote queries expose a serialized App.Error rather than an Error
  // instance. Its message must remain readable after crossing the wire.
  if (error && typeof error === "object") {
    if ("message" in error && typeof error.message === "string") return error.message;
    if (
      "body" in error &&
      error.body &&
      typeof error.body === "object" &&
      "message" in error.body &&
      typeof error.body.message === "string"
    )
      return error.body.message;
    return "The request failed. Try again.";
  }
  return String(error);
}
