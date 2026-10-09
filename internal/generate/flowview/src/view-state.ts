import type { SourcePage, ValueShape, ViewNode } from './types';

export interface ViewState {
  selected?: string;
  open: string[];
  details: string[];
  types: boolean;
  camera?: { x: number; y: number; zoom: number };
}

export interface SourceIndex {
  /** Exact presentation keys that can be selected in the current source. */
  selected: Map<string, ViewNode>;
  /** Exact presentation key to layout graph ID for expandable nodes/bundles. */
  expansionIDs: Map<string, string>;
  /** Exact presentation keys that can be opened. */
  open: Set<string>;
  /** Allowed disclosure paths, indexed by the selected presentation key. */
  details: Map<string, Set<string>>;
}

const VERSION = '1';
const MIN_ZOOM = 0.35;
const MAX_ZOOM = 1.8;
const MAX_COORDINATE = 1_000_000;
const NUMBER = /^-?(?:(?:0|[1-9]\d*)(?:\.\d+)?|\.\d+)(?:[eE][+-]?\d+)?$/;

type KeyedNode = ViewNode & { key?: string };
type KeyedBranch = NonNullable<ViewNode['branches']>[number] & { key?: string };

function keyOf(value: { key?: string } | undefined): string | undefined {
  return typeof value?.key === 'string' && value.key.length > 0 ? value.key : undefined;
}

function pointerPart(value: string): string {
  return value.replace(/~/g, '~0').replace(/\//g, '~1');
}

/** Return the exact disclosure paths rendered as expandable `<details>` by Shape.svelte. */
export function shapeDisclosurePaths(shape: ValueShape, path: string): string[] {
  const result: string[] = [];
  function visit(current: ValueShape, currentPath: string): void {
    const resolved = current.kind === 'pointer' && current.element ? current.element : current;
    const fields = resolved.fields ?? (resolved.kind === 'array' ? resolved.element?.fields : undefined);
    const hasArrayItem = !!(resolved.element && resolved.kind === 'array');
    if (!fields?.length && !hasArrayItem) return;
    result.push(currentPath);
    if (fields?.length) {
      for (const field of fields) visit(field.shape, `${currentPath}/${pointerPart(field.name)}`);
    } else if (resolved.element) {
      visit(resolved.element, `${currentPath}/item`);
    }
  }
  visit(shape, path);
  return result;
}

function detailPaths(node: ViewNode): Set<string> {
  const paths = new Set<string>(['/source']);
  if (node.kind === 'Set' || node.kind === 'SetJSON') {
    paths.add('/value');
    if (node.detail?.shape) {
      for (const path of shapeDisclosurePaths(node.detail.shape, '/shape')) paths.add(path);
    }
  } else {
    for (const field of node.context || []) {
      if (!field.shape) continue;
      for (const path of shapeDisclosurePaths(field.shape, `/context/${pointerPart(field.key)}`)) paths.add(path);
    }
  }
  return paths;
}

function addUnambiguous<K, V>(map: Map<K, V>, ambiguous: Set<K>, key: K, value: V): void {
  if (ambiguous.has(key)) return;
  if (map.has(key)) {
    map.delete(key);
    ambiguous.add(key);
    return;
  }
  map.set(key, value);
}

/**
 * Index source identities separately from layout's sequential graph IDs.
 * Missing or duplicate presentation keys are intentionally unavailable.
 */
export function indexSource(page: SourcePage): SourceIndex {
  const selected = new Map<string, ViewNode>();
  const selectedAmbiguous = new Set<string>();
  const details = new Map<string, Set<string>>();
  const open = new Set<string>();
  const expansionIDs = new Map<string, string>();
  const expansionAmbiguous = new Set<string>();
  const addSelected = (key: string | undefined, node: ViewNode): void => {
    if (!key) return;
    addUnambiguous(selected, selectedAmbiguous, key, node);
  };
  const addOpen = (key: string | undefined, graphID: string): void => {
    if (!key) return;
    addUnambiguous(expansionIDs, expansionAmbiguous, key, graphID);
  };

  const visit = (nodes: ViewNode[][] | undefined): void => {
    for (const sequence of nodes || []) {
      for (const node of sequence) {
        const keyed = node as KeyedNode;
        addSelected(keyOf(keyed), node);
        if (node.children?.length) addOpen(keyOf(keyed), node.id);

        if (node.kind === 'Condition') {
          const labels = node.branchLabels || [];
          const branches = node.branches || [];
          const isSwitch = node.control?.kind === 'switch' || node.control?.code.trimStart().startsWith('switch ');
          const defaultIndex = branches.findIndex((branch, index) => branch.default || labels[index] === 'else / default');
          for (const [index, rawBranch] of (node.branches || []).entries()) {
            if (isSwitch || index === 0 || index === defaultIndex) continue;
            const branch = rawBranch as KeyedBranch;
            if (!keyOf(branch)) continue;
            const synthetic: ViewNode = {
              ...node,
              id: `${node.id}-decision-${index}`,
              key: branch.key,
              title: branch.title,
              description: branch.description,
              source: branch.source || node.source,
              control: { kind: 'if', code: branch.code || branch.label || '' },
            } as KeyedNode;
            addSelected(keyOf(branch), synthetic);
          }
        }

        visit(node.children);
      }
      for (let i = 0; i < sequence.length; i++) {
        const node = sequence[i];
        if (node.kind !== 'Set' && node.kind !== 'SetJSON') continue;
        const firstKey = keyOf(node as KeyedNode);
        if (firstKey) addOpen(`${firstKey}:writes`, `${node.id}-writes`);
        while (i + 1 < sequence.length && ['Set', 'SetJSON'].includes(sequence[i + 1].kind)) i++;
      }
    }
  };
  visit([page.body]);

  for (const [key, node] of selected) details.set(key, detailPaths(node));
  for (const key of expansionAmbiguous) open.delete(key);
  for (const key of expansionIDs.keys()) open.add(key);
  return { selected, expansionIDs, open, details };
}

function parsedNumber(value: string | null): number | undefined {
  if (value === null || !NUMBER.test(value)) return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function roundedCamera(search: Pick<URLSearchParams, 'getAll'>): ViewState['camera'] {
  const xs = search.getAll('x');
  const ys = search.getAll('y');
  const zs = search.getAll('z');
  if (xs.length !== 1 || ys.length !== 1 || zs.length !== 1) return undefined;
  const x = parsedNumber(xs[0]);
  const y = parsedNumber(ys[0]);
  const zoom = parsedNumber(zs[0]);
  if (x === undefined || y === undefined || zoom === undefined) return undefined;
  if (Math.abs(x) > MAX_COORDINATE || Math.abs(y) > MAX_COORDINATE || zoom < MIN_ZOOM || zoom > MAX_ZOOM) return undefined;
  return { x: round(x, 2), y: round(y, 2), zoom: round(zoom, 3) };
}

function round(value: number, digits: number): number {
  const scale = 10 ** digits;
  return Math.round(value * scale) / scale;
}

export function normalizeView(state: ViewState, page: SourcePage): ViewState {
  const index = indexSource(page);
  const selectedValues = typeof state.selected === 'string' && index.selected.has(state.selected) ? state.selected : undefined;
  const allowedDetails = selectedValues ? index.details.get(selectedValues) : undefined;
  const details = new Set<string>();
  for (const path of state.details || []) {
    if (!allowedDetails?.has(path)) continue;
    details.add(path);
  }
  return {
    ...(selectedValues ? { selected: selectedValues } : {}),
    open: [...new Set((state.open || []).filter(key => index.open.has(key)))].sort(),
    details: [...details].sort(),
    types: state.types === true,
    ...(state.camera && validCamera(state.camera) ? { camera: canonicalCamera(state.camera) } : {}),
  };
}

function validCamera(camera: NonNullable<ViewState['camera']>): boolean {
  return Number.isFinite(camera.x) && Number.isFinite(camera.y) && Number.isFinite(camera.zoom) &&
    Math.abs(camera.x) <= MAX_COORDINATE && Math.abs(camera.y) <= MAX_COORDINATE &&
    camera.zoom >= MIN_ZOOM && camera.zoom <= MAX_ZOOM;
}

function canonicalCamera(camera: NonNullable<ViewState['camera']>): NonNullable<ViewState['camera']> {
  return { x: round(camera.x, 2), y: round(camera.y, 2), zoom: round(camera.zoom, 3) };
}

export function decodeView(search: URLSearchParams, page: SourcePage): ViewState {
  const versions = search.getAll('v');
  if (versions.length > 1 || (versions.length === 1 && versions[0] !== VERSION)) {
    return { open: [], details: [], types: false };
  }
  const selectedValues = [...new Set(search.getAll('selected'))];
  const state: ViewState = {
    ...(selectedValues.length === 1 ? { selected: selectedValues[0] } : {}),
    open: search.getAll('open'),
    details: search.getAll('detail'),
    types: search.getAll('types').length === 1 && search.get('types') === '1',
    camera: roundedCamera(search),
  };
  return normalizeView(state, page);
}

export function encodeView(state: ViewState): URLSearchParams {
  const search = new URLSearchParams();
  search.set('v', VERSION);
  if (typeof state.selected === 'string' && state.selected.length) search.set('selected', state.selected);
  for (const key of [...new Set(state.open || [])].filter(Boolean).sort()) search.append('open', key);
  if (state.selected) {
    for (const path of [...new Set(state.details || [])].filter(Boolean).sort()) search.append('detail', path);
  }
  if (state.types === true) search.set('types', '1');
  if (state.camera && validCamera(state.camera)) {
    const camera = canonicalCamera(state.camera);
    search.set('x', String(camera.x));
    search.set('y', String(camera.y));
    search.set('z', String(camera.zoom));
  }
  return search;
}
