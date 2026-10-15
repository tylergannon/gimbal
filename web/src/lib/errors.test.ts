import { expect, test } from "vite-plus/test";
import { error } from "@sveltejs/kit";
import { errorMessage } from "./errors";

test("serialized live-query failures display their server message", () => {
  expect(errorMessage({ message: "Not Found", status: 404 })).toBe("Not Found");
  expect(errorMessage({ body: { message: "The file changed. Reload before saving." } })).toBe(
    "The file changed. Reload before saving.",
  );
  expect(errorMessage({ status: 503 })).toBe("The request failed. Try again.");
});

test("ordinary and SvelteKit HTTP errors retain their existing messages", () => {
  expect(errorMessage(new Error("Connection lost"))).toBe("Connection lost");
  expect(errorMessage("SSH host is unreachable")).toBe("SSH host is unreachable");
  try {
    error(409, "The file changed");
  } catch (failure) {
    expect(errorMessage(failure)).toBe("The file changed");
  }
});
