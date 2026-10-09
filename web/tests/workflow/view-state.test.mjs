import assert from "node:assert/strict";
import test from "node:test";
import {
  decodeView,
  encodeView,
  indexSource,
  normalizeView,
  shapeDisclosurePaths,
} from "../../src/lib/workflow-design/view-state.ts";

const structured = (fields) => ({ type: "struct", kind: "struct", fields });
const leaf = (id, key, options = {}) => ({
  id,
  key,
  kind: "Generate",
  label: id,
  source: { file: "flow.go", line: 1 },
  scope: "",
  context: [],
  ...options,
});

const page = {
  name: "fixture",
  entry: "flow.go",
  module: "fixture",
  source: { file: "flow.go", line: 1 },
  body: [
    leaf("outer", "outer-key", {
      kind: "Group",
      children: [
        [
          leaf("inner", "hidden-key"),
          leaf("write-a", "write-a-key", {
            kind: "Set",
            detail: {
              kind: "value",
              expression: "x",
              promptKnown: false,
              keyKnown: true,
              contextKnown: false,
              shape: structured([
                {
                  name: "nested",
                  shape: structured([
                    { name: "field", shape: { type: "string", kind: "primitive" } },
                  ]),
                },
              ]),
            },
          }),
          leaf("write-b", "write-b-key", { kind: "SetJSON" }),
          leaf("after-write", "after-write-key"),
        ],
      ],
    }),
    leaf("decision", "decision-key", {
      kind: "Condition",
      control: { kind: "if", code: "if ok" },
      branches: [
        { key: "first-branch-key", code: "if ok", source: { file: "flow.go", line: 2 } },
        {
          key: "else-if-key",
          code: "else if next",
          title: "Next",
          source: { file: "flow.go", line: 3 },
        },
        {
          key: "default-key",
          code: "else",
          title: "Otherwise",
          default: true,
          source: { file: "flow.go", line: 4 },
        },
      ],
      children: [[leaf("yes", "yes-key")], [leaf("next", "next-key")], [leaf("no", "no-key")]],
    }),
    leaf("inspector", "inspector-key", {
      context: [
        {
          key: "outer/tilde~",
          shape: structured([
            {
              name: "child/key",
              shape: structured([{ name: "value", shape: { type: "string", kind: "primitive" } }]),
            },
          ]),
          write: "",
          scope: "",
        },
      ],
    }),
  ],
};

test("source index separates exact presentation keys from layout IDs", () => {
  const index = indexSource(page);
  assert.equal(index.selected.get("outer-key").id, "outer");
  assert.equal(index.selected.get("else-if-key").id, "decision-decision-1");
  assert.equal(
    index.selected.has("first-branch-key"),
    false,
    "the first condition is represented by the condition node key",
  );
  assert.equal(
    index.selected.has("default-key"),
    false,
    "the default branch has no selectable decision diamond",
  );
  assert.equal(index.expansionIDs.get("outer-key"), "outer");
  assert.equal(index.expansionIDs.get("write-a-key:writes"), "write-a-writes");
  assert.equal(index.open.has("hidden-key"), false, "leaf nodes are not expandable");
  assert(
    index.selected.has("hidden-key"),
    "hidden descendants remain valid selectable source nodes",
  );
  assert(!index.selected.has(undefined));

  const switchPage = structuredClone(page);
  switchPage.body[1].control = { kind: "switch", code: "switch value" };
  switchPage.body[1].branches.forEach((branch, index) => {
    branch.key = `case-${index}`;
    branch.default = index === 2;
  });
  const switchIndex = indexSource(switchPage);
  assert.equal(
    switchIndex.selected.has("case-0"),
    false,
    "switch cases are not separate selectable decision nodes",
  );
  assert.equal(
    switchIndex.selected.has("case-1"),
    false,
    "switch cases are not separate selectable decision nodes",
  );
});

test("shape disclosure paths mirror Shape.svelte and escape JSON Pointer segments", () => {
  const fields = structured([
    {
      name: "child/key~",
      shape: structured([{ name: "leaf", shape: { type: "string", kind: "primitive" } }]),
    },
  ]);
  assert.deepEqual(shapeDisclosurePaths(fields, "/shape"), ["/shape", "/shape/child~1key~0"]);

  const arrayWithInlineFields = { type: "[]Thing", kind: "array", element: fields };
  assert.deepEqual(shapeDisclosurePaths(arrayWithInlineFields, "/context/items"), [
    "/context/items",
    "/context/items/child~1key~0",
  ]);

  const nestedArray = {
    type: "[][]string",
    kind: "array",
    element: { type: "[]string", kind: "array", element: { type: "string", kind: "primitive" } },
  };
  assert.deepEqual(shapeDisclosurePaths(nestedArray, "/context/items"), [
    "/context/items",
    "/context/items/item",
  ]);
});

test("decode keeps exact known keys and disclosure paths without adding ancestors", () => {
  const decoded = decodeView(
    new URLSearchParams([
      ["v", "1"],
      ["selected", "inspector-key"],
      ["open", "outer-key"],
      ["open", "outer-key"],
      ["open", "missing"],
      ["open", "write-a-key:writes"],
      ["detail", "/context/outer~1tilde~0/child~1key"],
      ["detail", "/source"],
      ["detail", "/source/typo"],
      ["types", "1"],
      ["x", "-2.126"],
      ["y", "4"],
      ["z", "1.7777"],
      ["unrecognized", "anything"],
    ]),
    page,
  );
  assert.deepEqual(decoded, {
    selected: "inspector-key",
    open: ["outer-key", "write-a-key:writes"],
    details: ["/context/outer~1tilde~0/child~1key", "/source"],
    types: true,
    camera: { x: -2.13, y: 4, zoom: 1.778 },
  });
});

test("decode drops removed/ambiguous identities without losing independent valid state", () => {
  const changed = structuredClone(page);
  changed.body[0].key = "duplicate";
  changed.body[1].key = "duplicate";
  const decoded = decodeView(
    new URLSearchParams(
      "selected=duplicate&open=write-a-key%3Awrites&open=missing&types=1&detail=%2Fsource",
    ),
    changed,
  );
  assert.deepEqual(decoded, { open: ["write-a-key:writes"], details: [], types: true });
});

test("missing version means v1; unsupported or ambiguous versions reset presentation choices", () => {
  const missing = decodeView(new URLSearchParams("selected=inspector-key&types=1"), page);
  assert.equal(missing.selected, "inspector-key");
  assert.equal(missing.types, true);
  assert.deepEqual(
    decodeView(new URLSearchParams("v=9&selected=inspector-key&open=outer-key&types=1"), page),
    {
      open: [],
      details: [],
      types: false,
    },
  );
  assert.deepEqual(decodeView(new URLSearchParams("v=1&v=1&selected=inspector-key"), page), {
    open: [],
    details: [],
    types: false,
  });
});

test("invalid selected keys clear their inspector-only disclosures", () => {
  assert.deepEqual(
    decodeView(new URLSearchParams("selected=removed&detail=%2Fsource&open=outer-key"), page),
    {
      open: ["outer-key"],
      details: [],
      types: false,
    },
  );
});

test("camera requires one complete strict finite tuple within bounds", () => {
  for (const query of [
    "x=1&y=2",
    "x=1&x=2&y=2&z=1",
    "x=NaN&y=2&z=1",
    "x=Infinity&y=2&z=1",
    "x=0x10&y=2&z=1",
    "x=2junk&y=2&z=1",
    "x=1000000.01&y=2&z=1",
    "x=1&y=2&z=0.349",
    "x=1&y=2&z=1.801",
  ])
    assert.equal(decodeView(new URLSearchParams(query), page).camera, undefined, query);

  assert.deepEqual(decodeView(new URLSearchParams("x=-1000000&y=1000000&z=.35"), page).camera, {
    x: -1000000,
    y: 1000000,
    zoom: 0.35,
  });
});

test("encoder and decoder produce a canonical stable round trip", () => {
  const input = new URLSearchParams([
    ["v", "1"],
    ["selected", "inspector-key"],
    ["open", "outer-key"],
    ["open", "outer-key"],
    ["detail", "/context/outer~1tilde~0/child~1key"],
    ["types", "1"],
    ["x", "2.124"],
    ["y", "-3.456"],
    ["z", "1.23456"],
    ["stray", "drop"],
  ]);
  const state = decodeView(input, page);
  const canonical = encodeView(state);
  assert.equal(
    canonical.toString(),
    "v=1&selected=inspector-key&open=outer-key&detail=%2Fcontext%2Fouter%7E1tilde%7E0%2Fchild%7E1key&types=1&x=2.12&y=-3.46&z=1.235",
  );
  assert.deepEqual(decodeView(canonical, page), state);
  assert.equal(encodeView(decodeView(canonical, page)).toString(), canonical.toString());
});

test("normalizeView retains valid disclosures and drops invalid camera and values", () => {
  const normalized = normalizeView(
    {
      selected: "yes-key",
      open: ["outer-key", "unknown"],
      details: ["/source"],
      types: false,
      camera: { x: 0, y: 0, zoom: 9 },
    },
    page,
  );
  assert.deepEqual(normalized, {
    selected: "yes-key",
    open: ["outer-key"],
    details: ["/source"],
    types: false,
  });
});

test("collapsing a disclosure parent retains its child path without reopening the parent", () => {
  const parent = "/context/outer~1tilde~0";
  const child = `${parent}/child~1key`;
  const initiallyOpen = decodeView(
    new URLSearchParams([
      ["selected", "inspector-key"],
      ["detail", parent],
      ["detail", child],
    ]),
    page,
  );
  assert.deepEqual(initiallyOpen.details, [parent, child]);

  const collapsed = normalizeView(
    { ...initiallyOpen, details: initiallyOpen.details.filter((path) => path !== parent) },
    page,
  );
  assert.deepEqual(collapsed.details, [child]);
  const restored = decodeView(encodeView(collapsed), page);
  assert.deepEqual(restored.details, [child]);
  assert(
    !restored.details.includes(parent),
    "the child path must not implicitly reopen its collapsed ancestor",
  );
});

test("encoder omits defaults and incomplete camera values, always includes v1", () => {
  assert.equal(
    encodeView({
      open: [],
      details: ["/source"],
      types: false,
      camera: { x: 1, y: 2, zoom: 9 },
    }).toString(),
    "v=1",
  );
  assert.equal(
    encodeView({ selected: "x", open: [], details: ["/source"], types: false }).toString(),
    "v=1&selected=x&detail=%2Fsource",
  );
});

test("workflow guide/run presentation shares exact URL state and rejects invalid panels", () => {
  const state = decodeView(
    new URLSearchParams("panel=guide&selected=inner-key&open=outer-key&types=1"),
    page,
  );
  assert.equal(state.panel, "guide");
  assert.equal(state.selected, undefined);
  assert.deepEqual(state.open, ["outer-key"]);
  assert.equal(encodeView(state).get("panel"), "guide");
  assert.equal(decodeView(new URLSearchParams("panel=wrong"), page).panel, undefined);
  assert.equal(decodeView(new URLSearchParams("panel=run&panel=guide"), page).panel, undefined);
});
