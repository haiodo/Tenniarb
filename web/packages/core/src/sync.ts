// Port of SyncExtension.swift toSyncJson (Element -> JSON of items and edges).
import { LinkItem, itemCommandName } from "./element-model.ts";
import type { Element } from "./element-model.ts";
import type { TennNode } from "./model.ts";
import { toStr } from "./printer.ts";

const nl = (s: string): string => s.replaceAll("\n", "\\n");
const propSync = (n: TennNode): string[] => (n.children ?? []).map((c) => nl(toStr(c, 0, true)));

/** JSONEncoder .prettyPrinted layout: `"key" : value`, escaped slashes, absent keys for nil. */
export function toSyncJson(el: Element): string {
  const items: object[] = [];
  const edges: object[] = [];
  for (const it of el.items) {
    const link = it instanceof LinkItem && it.source !== null && it.target !== null ? it : null;
    const props = [...it.properties];
    (link === null ? items : edges).push({
      kind: itemCommandName(it.kind),
      name: nl(it.name),
      id: it.id,
      pos: { x: it.x, y: it.y },
      description: it.description === null ? undefined : nl(it.description),
      properties: props.length === 0 ? undefined : props.map(propSync),
      source: link?.source?.id,
      target: link?.target?.id,
    });
  }
  const json = JSON.stringify({ name: el.name, description: el.description ?? undefined, items: items.length > 0 ? items : undefined, edges: edges.length > 0 ? edges : undefined }, null, 2);
  return json.replace(/^(\s*"(?:[^"\\]|\\.)*"): /gm, "$1 : ").replaceAll("/", "\\/");
}
