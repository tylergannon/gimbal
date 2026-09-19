import type { Tokens, Total, Usage } from "../../observation/index.js";

/** A wall clock the fixtures share. The specimens are drawn at fixed times
 * of day, so the fixtures state them in UTC and never read the real clock:
 * a story looks the same on every machine and in every test run. */
export const at = (utc: string): number => Date.parse(utc + "Z");

/** Zero of every count, the shape a provider that reported nothing leaves. */
export const noTokens = (): Tokens => ({
  input: 0,
  cache_read: 0,
  cache_write: 0,
  output: 0,
  reasoning: 0,
});

/** The counts a harness reported, with the ones it did not report left at 0.
 * `stated_cost` is 0 when the harness stated no cost, which is what every
 * run in these fixtures did. */
export const usage = (counts: Partial<Usage>): Usage => ({
  ...noTokens(),
  stated_cost: 0,
  ...counts,
});

/** One roll-up: the same counts across every model and under the one model
 * that produced them. Go computes these; the browser never sums anything. */
export const total = (model: string, counts: Partial<Usage>): Total => {
  const value = usage(counts);
  return { all: value, by_model: { [model]: value } };
};
