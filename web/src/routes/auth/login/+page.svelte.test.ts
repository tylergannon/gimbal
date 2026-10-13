import { beforeEach, expect, test, vi } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import Page from "./+page.svelte";

const login = vi.hoisted(() => ({
  page: { data: { authEnabled: true } },
  begin: Object.assign(vi.fn(), { pending: 0 }),
}));
vi.mock("$app/state", () => ({ page: login.page }));
vi.mock("../../auth.remote", () => ({ beginLogin: login.begin }));

beforeEach(() => {
  login.page.data.authEnabled = true;
  login.begin.mockReset();
});

test("signed-out visitors get the GitHub sign-in button", async () => {
  const screen = await render(Page);
  await expect.element(screen.getByRole("button", { name: "Sign in with GitHub" })).toBeVisible();
  expect(login.begin).not.toHaveBeenCalled();
});

test("a failed login command remains visible and can be retried", async () => {
  login.begin.mockRejectedValue(new Error("Could not start sign-in."));
  const screen = await render(Page);
  await screen.getByRole("button", { name: "Sign in with GitHub" }).click();
  await expect.element(screen.getByRole("alert")).toHaveTextContent("Could not start sign-in.");
  await screen.getByRole("button", { name: "Sign in with GitHub" }).click();
  expect(login.begin).toHaveBeenCalledTimes(2);
});
