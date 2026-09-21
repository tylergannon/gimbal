import { createSubscriber } from "svelte/reactivity";

const subscribe = createSubscriber((update) => {
  const timer = setInterval(update, 1000);
  return () => clearInterval(timer);
});

/** Wall-clock time that ticks once a second while anything reads it. One
 * interval for the whole app, running only while it has readers. */
export const clock = {
  get now() {
    subscribe();
    return Date.now();
  },
};
