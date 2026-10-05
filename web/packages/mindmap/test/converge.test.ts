// Two Yjs clients edit the same block concurrently through the text protocol, then sync: texts and models must agree.
// MINDMAP_SEEDS=2000 node --test packages/mindmap/test/converge.test.ts for a longer run.
import assert from "node:assert/strict";
import { test } from "node:test";
import * as Y from "yjs";
import "../../render/test/helpers.ts"; // measure context + fonts
import { MindmapDoc } from "../src/doc.ts";
import { keys, parseBlock, printItems, writeBlock } from "../src/text.ts";

const SRC = `item "Root" {
    pos 0 0
    color orange
}
item "A" {
    pos 160 40
}
item "B" {
    pos 160 -40
}
link "Root" "A"
link "Root" "B" {
    display arrow
}
`;

// mulberry32
function rng(seed: number): () => number {
  return () => {
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const STYLES: [string, string[]][] = [
  ["color", ["red", "green", "blue", "yellow"]],
  ["display", ["rect", "circle", "no-fill"]],
  ["font-size", ["12", "18", "26"]],
  ["text-color", ["black", "purple"]],
];

function client(id: number) {
  const ydoc = new Y.Doc();
  ydoc.clientID = id;
  const ytext = ydoc.getText("block");
  const mm = new MindmapDoc("", {
    onChange: (changes) =>
      ydoc.transact(() => {
        for (const c of [...changes].reverse()) {
          ytext.delete(c.from, c.to - c.from);
          ytext.insert(c.from, c.insert);
        }
      }),
  });
  mm.session.undoManager.groupsByEvent = false;
  // Own transactions come back here too and are no-ops: the text is the doc's already.
  ytext.observe(() => mm.update(ytext.toString()));
  return { ydoc, ytext, mm };
}

type Client = ReturnType<typeof client>;
const sync = (a: Client, b: Client): void => {
  const [ua, ub] = [Y.encodeStateAsUpdate(a.ydoc, Y.encodeStateVector(b.ydoc)), Y.encodeStateAsUpdate(b.ydoc, Y.encodeStateVector(a.ydoc))];
  Y.applyUpdate(b.ydoc, ua);
  Y.applyUpdate(a.ydoc, ub);
};

const stats = { ops: 0, skipped: 0, dangling: 0, untidy: 0, dupNames: 0 };

function randomOp(c: Client, r: () => number, adds: { n: number }): void {
  const s = c.mm.session;
  const items = s.element.items.filter((i) => i.kind === "Item");
  const pick = <T>(list: T[]): T => list[Math.floor(r() * list.length)]!;
  const centre = (i: (typeof items)[number]) => {
    const b = s.scene.drawables.get(i)!.getSelectorBounds();
    return { x: b.x + b.width / 2, y: b.y + b.height / 2 };
  };
  const op = items.length === 0 ? 2 : Math.floor(r() * 7);
  stats.ops++;
  if (op === 0) {
    const p = centre(pick(items));
    s.down(p);
    const to = { x: p.x + (r() - 0.5) * 300, y: p.y + (r() - 0.5) * 300 };
    s.move({ x: (p.x + to.x) / 2, y: (p.y + to.y) / 2 });
    s.up(to);
  } else if (op === 1) {
    // A small pool, so that names collide now and then.
    s.commitEdit(pick(items), "name", `N${Math.floor(r() * 12)}`);
  } else if (op === 2) {
    if (items.length > 0 && r() < 0.8) {
      const p = centre(pick(items));
      s.down(p);
      s.up(p);
    } else s.select([]);
    s.addNewItem();
    adds.n++;
  } else if (op === 3) {
    s.select([pick(s.element.items)]);
    s.deleteSelection();
  } else if (op === 4) {
    const [prop, options] = pick(STYLES);
    s.select([pick(items)]);
    s.setQuickStyle(prop, pick(options));
  } else if (op === 6) {
    // Only edits made since the last remote change are left to undo.
    s.undo();
  } else if (items.length > 1) {
    const [a, b] = [pick(items), pick(items)];
    if (a === b) return;
    s.down(centre(a), { line: true });
    s.move(centre(b));
    s.up(centre(b));
  }
}

function check(a: Client, b: Client, seed: number, final = false): void {
  const text = a.ytext.toString();
  assert.equal(b.ytext.toString(), text, `seed ${seed}: Yjs texts`);
  assert.equal(a.mm.text, text, `seed ${seed}: client 1 saw the merge`);
  assert.equal(b.mm.text, text, `seed ${seed}: client 2 saw the merge`);
  const block = parseBlock(text);
  // An unchanged model writes the text back as it is: nothing is rewritten behind the user's back.
  assert.equal(writeBlock(block, block.items), text, `seed ${seed}: idempotent write`);
  for (const line of text.split("\n")) {
    if (/^\s*pos\b/.test(line)) assert.match(line, /^ {4}pos -?\d+ -?\d+$/, `seed ${seed}: pos line`);
  }
  const model = (c: Client) => {
    const items = c.mm.session.element.items;
    const printed = printItems(items);
    return keys(items).map((k, n) => `${k}=${printed.get(items[n]!)}`);
  };
  assert.deepEqual(model(a), model(b), `seed ${seed}: models`);
  assert.deepEqual(model(a), keys(block.items).map((k, n) => `${k}=${printItems(block.items).get(block.items[n]!)}`), `seed ${seed}: model is the text`);
  const headers = text.split("\n").filter((l) => l.startsWith("item ")).length;
  const links = text.split("\n").filter((l) => l.startsWith("link ")).length;
  const shown = block.items.filter((i) => i.kind === "Item");
  assert.ok(shown.length <= headers, `seed ${seed}: items`);
  if (!final) return;
  stats.skipped += headers - shown.length;
  stats.dangling += links - (block.items.length - shown.length);
  stats.untidy += block.regions.filter((r) => r.items.length > 0 && r.text !== block.lines.slice(r.start, r.end).join("\n")).length;
  stats.dupNames += shown.length - new Set(shown.map((i) => i.name)).size;
}

test("convergence: two Yjs clients, random concurrent edits, sync", (t) => {
  const seeds = Number(process.env.MINDMAP_SEEDS ?? 60);
  for (let seed = 1; seed <= seeds; seed++) {
    const r = rng(seed);
    const [a, b] = [client(1), client(2)];
    a.ytext.insert(0, SRC);
    sync(a, b);
    const adds = { n: 0 };
    for (let round = 0; round < 12; round++) {
      for (const c of [a, b]) for (let k = Math.floor(r() * 3) + 1; k > 0; k--) randomOp(c, r, adds);
      sync(a, b);
      check(a, b, seed, round === 11);
      assert.ok(a.mm.session.element.items.filter((i) => i.kind === "Item").length <= 3 + adds.n, `seed ${seed}: no items out of nowhere`);
    }
    // One more edit of every item, without concurrency, rewrites its region: duplicates and orphans are gone (links are not edited).
    for (const i of a.mm.session.element.items.filter((x) => x.kind === "Item")) {
      a.mm.session.select([i]);
      a.mm.session.moveBy(1, 0);
    }
    sync(a, b);
    check(a, b, seed);
    if (process.env.MINDMAP_DUMP === String(seed)) console.log(a.ytext.toString());
    const block = parseBlock(a.ytext.toString());
    const untidy = block.regions.filter((r) => r.items.some((i) => i.kind === "Item") && r.text !== block.lines.slice(r.start, r.end).join("\n"));
    assert.deepEqual(untidy.map((r) => block.lines.slice(r.start, r.end).join("\n")), [], `seed ${seed}: tidy`);
  }
  t.diagnostic(`seeds ${seeds}: ${JSON.stringify(stats)}`);
});
