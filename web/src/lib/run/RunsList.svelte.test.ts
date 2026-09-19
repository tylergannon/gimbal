import { expect, test } from "vite-plus/test";
import { render } from "vitest-browser-svelte";
import RunsList from "./RunsList.svelte";
import { runsListFixture } from "./fixtures/index.js";

test("cards show observation-derived facts and open the selected run", async () => {
  const opened: string[] = [];
  const screen = await render(RunsList, {
    items: runsListFixture.items,
    attention: runsListFixture.attention,
    now: 1_800_000_760_000,
    onopenrun: (run) => opened.push(run.id),
  });

  const card = screen.getByRole("button", {
    name: "Open plan-trip 01M2RWXNFFYQA4HHQWMQ252CYF",
  });
  await expect.element(card).toHaveTextContent("Running");
  await expect.element(card).toHaveTextContent("Latest activity");
  await expect.element(card).toHaveTextContent("40 s");
  await expect.element(card).toHaveTextContent("$0.32");
  await expect.element(card).toHaveTextContent("12m 40s");
  await expect.element(card).toHaveTextContent("Sessions3");
  await expect.element(card).toHaveTextContent("Turns7");
  await expect
    .element(card)
    .toHaveTextContent("Compare the available cabins and ask about the tradeoffs that matter.");

  await card.click();
  expect(opened).toEqual(["01M2RWXNFFYQA4HHQWMQ252CYF"]);
  await expect.element(screen.getByText("No instruction recorded yet.")).toBeVisible();
});

test("filters, search, attention access, and refreshed card data stay useful", async () => {
  const opened: string[] = [];
  const screen = await render(RunsList, {
    items: runsListFixture.items,
    attention: runsListFixture.attention,
    now: 1_800_000_760_000,
    onopenrun: (run) => opened.push(run.id),
  });

  await screen.getByRole("button", { name: "Failed · 1" }).click();
  await expect
    .element(
      screen.getByRole("button", {
        name: "Open implement-interview 01M2QZ7PB3N6D0R9X2G5HKW8VY",
      }),
    )
    .toBeVisible();
  await expect
    .element(
      screen.getByRole("button", {
        name: "Open plan-trip 01M2RWXNFFYQA4HHQWMQ252CYF",
      }),
    )
    .not.toBeInTheDocument();

  await screen.getByRole("button", { name: "All · 6" }).click();
  await screen.getByPlaceholder("Search by workflow or run id").fill("Q1HD4");
  await expect
    .element(
      screen.getByRole("button", {
        name: "Open plan-trip 01M2Q1HD4T8KJ2M7P5S0B9XNAE",
      }),
    )
    .toBeVisible();

  await screen.getByPlaceholder("Search by workflow or run id").fill("");
  await screen.getByText("Would you trade reliable Wi-Fi for a more secluded cabin?").click();
  expect(opened).toEqual(["01M2RWXNFFYQA4HHQWMQ252CYF"]);

  const refreshed = structuredClone(runsListFixture.items);
  refreshed[0].activity_at = 1_800_000_755_000;
  refreshed[0].cost = "$0.44";
  refreshed[0].turn_count = 8;
  refreshed[0].instruction = "Use the newly answered preference to rank the final options.";
  await screen.rerender({
    items: refreshed,
    attention: runsListFixture.attention,
    now: 1_800_000_760_000,
    onopenrun: (run) => opened.push(run.id),
  });

  const refreshedCard = screen.getByRole("button", {
    name: "Open plan-trip 01M2RWXNFFYQA4HHQWMQ252CYF",
  });
  await expect.element(refreshedCard).toHaveTextContent("5 s");
  await expect.element(refreshedCard).toHaveTextContent("$0.44");
  await expect.element(refreshedCard).toHaveTextContent("Turns8");
  await expect
    .element(refreshedCard)
    .toHaveTextContent("Use the newly answered preference to rank the final options.");
});
