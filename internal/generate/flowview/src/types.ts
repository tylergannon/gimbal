import type { Edge, Node } from '@xyflow/svelte';

export interface Source { file: string; line: number; column?: number }
export interface ValueShape {
  type: string;
  kind: string;
  fields?: { name: string; shape: ValueShape; optional?: boolean; tag?: string }[];
  element?: ValueShape;
  note?: string;
  goOnly?: boolean;
}
export interface ContextField {
  key: string; dynamic?: boolean; shape?: ValueShape; write: string; scope: string;
  when?: string; optional?: boolean; shadowed?: string[];
}
export interface ViewNode {
  id: string; kind: string; label: string; title?: string; description?: string;
  source: Source; scope: string; session?: string; from?: string; prompt?: string;
  detail?: {
    kind: string; expression: string; promptExpression?: string; promptKnown: boolean;
    keyExpression?: string; keyKnown: boolean; valueExpression?: string;
    contextExpression?: string; contextKnown: boolean; shape?: ValueShape;
  };
  control?: { code: string; kind?: string; actions?: string[] };
  branches?: { code: string; source: Source; title?: string; description?: string; label?: string; default?: boolean }[];
  context: ContextField[];
  children?: ViewNode[][];
  branchLabels?: string[];
  branchTitles?: string[];
  branchDescriptions?: string[];
  branchExits?: boolean[];
  note?: string;
}
export interface SourcePage {
  name: string; entry: string; module: string; source: Source; body: ViewNode[];
  diagnostics?: { message?: string; [key: string]: unknown }[];
}
export interface Port { id: string; x: number; y: number; side: 'NORTH' | 'SOUTH' | 'EAST' | 'WEST' }
export type WorkflowData = {
  label: string; kind: string; sourceNode?: ViewNode; expanded?: boolean;
  expandable?: boolean; members?: ViewNode[]; ports?: Port[]; width: number; height: number;
  [key: string]: unknown;
};
export interface Point { x: number; y: number }
export type RoutedData = {
  sections: { startPoint: Point; bendPoints?: Point[]; endPoint: Point }[];
  labels: { text: string; x: number; y: number; width: number; height: number }[];
  [key: string]: unknown;
};
export type WorkflowNode = Node<WorkflowData, 'workflow' | 'junction'>;
export type WorkflowEdge = Edge<RoutedData, 'routed'>;
