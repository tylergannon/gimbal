import assert from "node:assert/strict";
import { afterEach, test } from "vite-plus/test";
import {
  DEFAULT_WIDTH,
  MIN_WIDTH,
  clampWidth,
  readStoredMaximized,
  readStoredWidth,
  storeMaximized,
  storeWidth,
} from "./paneSize.js";

// This file runs under the plain node test project (no DOM), matching the
// SSR environment the guards are written for. A tiny in-memory Storage
// stand-in lets the round-trip tests exercise the real read/write paths.
class MemoryStorage {
  #store = new Map<string, string>();
  getItem(key: string): string | null {
    return this.#store.has(key) ? (this.#store.get(key) as string) : null;
  }
  setItem(key: string, value: string): void {
    this.#store.set(key, String(value));
  }
  removeItem(key: string): void {
    this.#store.delete(key);
  }
  clear(): void {
    this.#store.clear();
  }
}

const originalLocalStorage = (globalThis as { localStorage?: unknown }).localStorage;

afterEach(() => {
  if (originalLocalStorage === undefined) {
    delete (globalThis as { localStorage?: unknown }).localStorage;
  } else {
    (globalThis as { localStorage?: unknown }).localStorage = originalLocalStorage;
  }
});

test("clampWidth keeps a width inside the drag range", () => {
  assert.equal(clampWidth(480, 1280), 480);
  assert.equal(clampWidth(0, 1280), MIN_WIDTH);
  assert.equal(clampWidth(10_000, 1280), 1280 - 360);
});

test("clampWidth never drops below the minimum even on a narrow viewport", () => {
  assert.equal(clampWidth(500, 500), MIN_WIDTH);
  assert.equal(clampWidth(200, 500), MIN_WIDTH);
});

test("without storage (SSR) reads fall back to defaults and writes are silent", () => {
  delete (globalThis as { localStorage?: unknown }).localStorage;
  assert.equal(readStoredWidth(), DEFAULT_WIDTH);
  assert.equal(readStoredMaximized(), false);
  assert.doesNotThrow(() => storeWidth(600));
  assert.doesNotThrow(() => storeMaximized(true));
});

test("readStoredWidth defaults when the stored value is invalid", () => {
  (globalThis as { localStorage: Storage }).localStorage =
    new MemoryStorage() as unknown as Storage;
  localStorage.setItem("gimbal.detail.width", "not-a-number");
  assert.equal(readStoredWidth(), DEFAULT_WIDTH);
});

test("storeWidth and readStoredWidth round-trip", () => {
  (globalThis as { localStorage: Storage }).localStorage =
    new MemoryStorage() as unknown as Storage;
  storeWidth(620);
  assert.equal(readStoredWidth(), 620);
});

test("readStoredMaximized defaults to false and round-trips", () => {
  (globalThis as { localStorage: Storage }).localStorage =
    new MemoryStorage() as unknown as Storage;
  assert.equal(readStoredMaximized(), false);
  storeMaximized(true);
  assert.equal(readStoredMaximized(), true);
  storeMaximized(false);
  assert.equal(readStoredMaximized(), false);
});

test("storeWidth tolerates storage that throws", () => {
  const throwing = {
    getItem() {
      return null;
    },
    setItem() {
      throw new Error("quota exceeded");
    },
  };
  (globalThis as { localStorage: unknown }).localStorage = throwing;
  assert.doesNotThrow(() => storeWidth(500));
});
