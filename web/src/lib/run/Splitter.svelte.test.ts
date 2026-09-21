import { expect, test } from "vite-plus/test";
import { userEvent } from "vitest/browser";
import { render } from "vitest-browser-svelte";
import Splitter from "./Splitter.svelte";

test("arrow keys move the width by 24px and report both resize and commit", async () => {
  const resized: number[] = [];
  const committed: number[] = [];
  const screen = await render(Splitter, {
    width: 480,
    min: 380,
    max: 900,
    onresize: (width: number) => resized.push(width),
    oncommit: (width: number) => committed.push(width),
  });
  const handle = screen.getByRole("separator");
  await expect.element(handle).toHaveAttribute("aria-valuenow", "480");
  await expect.element(handle).toHaveAttribute("aria-valuemin", "380");
  await expect.element(handle).toHaveAttribute("aria-valuemax", "900");

  await handle.click();
  await userEvent.keyboard("{ArrowLeft}");
  expect(resized.at(-1)).toBe(504);
  expect(committed.at(-1)).toBe(504);

  // Splitter is presentational: it reports the next width but does not own
  // it, so the caller must feed the updated width back in.
  await screen.rerender({
    width: 504,
    min: 380,
    max: 900,
    onresize: (width: number) => resized.push(width),
    oncommit: (width: number) => committed.push(width),
  });
  await userEvent.keyboard("{ArrowRight}");
  expect(resized.at(-1)).toBe(480);
  expect(committed.at(-1)).toBe(480);
});

test("Home and End jump to the min and max", async () => {
  const resized: number[] = [];
  const screen = await render(Splitter, {
    width: 480,
    min: 380,
    max: 900,
    onresize: (width: number) => resized.push(width),
  });
  const handle = screen.getByRole("separator");

  await handle.click();
  await userEvent.keyboard("{Home}");
  expect(resized.at(-1)).toBe(380);

  await userEvent.keyboard("{End}");
  expect(resized.at(-1)).toBe(900);
});
