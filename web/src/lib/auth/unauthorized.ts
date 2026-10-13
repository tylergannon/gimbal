import { goto } from "$app/navigation";
import { isHttpError } from "@sveltejs/kit";

/** The operation was refused; return to sign-in without replaying it. */
export function navigateOnUnauthorized(error: unknown): boolean {
  if (!isHttpError(error, 401)) return false;
  void goto("/auth/login");
  return true;
}
