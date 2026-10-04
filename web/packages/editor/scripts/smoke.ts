// Playwright smoke: node scripts/smoke.ts [outDir]. Not part of CI; Playwright comes from web/bench/node_modules.
// Needs dist/ (npm run build -w @tenniarb/editor).
import { createServer } from "node:http";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { extname, join, normalize } from "node:path";

const out = process.argv[2] ?? ".work/editor";
mkdirSync(out, { recursive: true });
const dist = new URL("../dist/", import.meta.url).pathname;
const { chromium } = createRequire(import.meta.url)("../../../bench/node_modules/playwright");

const types: Record<string, string> = { ".html": "text/html", ".js": "text/javascript", ".woff2": "font/woff2" };
const server = createServer((req, res) => {
  const file = normalize(join(dist, decodeURIComponent(new URL(req.url ?? "/", "http://x").pathname)));
  try {
    if (!file.startsWith(dist)) throw new Error("outside");
    res.setHeader("content-type", types[extname(file)] ?? "application/octet-stream");
    res.end(readFileSync(file));
  } catch {
    res.statusCode = 404;
    res.end("not found");
  }
}).listen(0, "127.0.0.1");
await new Promise((r) => server.once("listening", r));
const base = `http://127.0.0.1:${(server.address() as { port: number }).port}`;

// Empty canvas points (the canvas is right of the outline and above the properties panel).
const EMPTY = { x: 1450, y: 640 };
const failures: string[] = [];
const check = (ok: boolean, what: string): void => {
  console.log(`${ok ? "ok  " : "FAIL"} ${what}`);
  if (!ok) failures.push(what);
};

const browser = await chromium.launch();
const p = await browser.newPage({ viewport: { width: 1500, height: 800 }, deviceScaleFactor: 2 });
const problems: string[] = [];
p.on("console", (m: { type(): string; text(): string }) => ["error", "warning"].includes(m.type()) && problems.push(m.text()));
p.on("pageerror", (e: Error) => problems.push(String(e)));
await p.goto(`${base}/demo.html`);
await p.waitForFunction(() => (window as any).editor !== undefined);
await p.waitForTimeout(500);

// Scene state the checks compare: first item position and both ends of the first link.
const state = () =>
  p.evaluate(() => {
    const s = (window as any).editor.session;
    const item = s.element.items[0];
    const link = s.element.items.find((i: any) => i.kind === "Link");
    const l = s.scene.drawables.get(link);
    return { x: item.x, y: item.y, src: l.source, dst: l.target, changes: (window as any).changes, sel: s.selection.length };
  });
const at = () => p.evaluate(() => { const e = (window as any).editor; return e.screenOf(e.session.element.items[0]); });

const ink = await p.$eval("canvas", (c: HTMLCanvasElement) => c.getContext("2d")!.getImageData(0, 0, c.width, c.height).data.some((v, i) => i % 4 === 3 && v !== 0));
check(ink, "canvas is drawn");
check(problems.length === 0, `no console errors ${problems.slice(0, 3).join(" | ")}`);
await p.screenshot({ path: `${out}/1-before.png` });
const s0 = await state();

const pt = await at();
await p.mouse.click(pt.x, pt.y);
await p.waitForTimeout(100);
check((await state()).sel === 1, "click selects the item");
await p.screenshot({ path: `${out}/2-selected.png` });

await p.mouse.move(pt.x, pt.y);
await p.mouse.down();
await p.mouse.move(pt.x + 80, pt.y + 60, { steps: 6 });
await p.mouse.up();
await p.waitForTimeout(100);
const s1 = await state();
check(s1.x > s0.x && s1.y < s0.y, `drag moved the model (${s0.x},${s0.y}) -> (${s1.x},${s1.y})`);
check(JSON.stringify([s1.src, s1.dst]) !== JSON.stringify([s0.src, s0.dst]), "link re-routed");
check(s1.changes === 1, `onChange fired once (${s1.changes})`);
check((await p.evaluate(() => (window as any).lastText)).includes(`pos ${s1.x}`), "onChange text carries the new position");
await p.screenshot({ path: `${out}/3-dragged.png` });

await p.keyboard.press("Control+z");
await p.waitForTimeout(100);
const s2 = await state();
check(s2.x === s0.x && s2.y === s0.y && JSON.stringify([s2.src, s2.dst]) === JSON.stringify([s0.src, s0.dst]), "ctrl+z restores item and link");
check(s2.changes === 2, `onChange fired on undo (${s2.changes})`);
await p.screenshot({ path: `${out}/4-undone.png` });
await p.keyboard.press("Control+Shift+z");
await p.waitForTimeout(100);
const s3 = await state();
check(s3.x === s1.x && s3.y === s1.y, "ctrl+shift+z redoes");

// empty click clears; drag on empty pans without touching the model
await p.mouse.click(EMPTY.x, EMPTY.y);
check((await state()).sel === 0, "click on empty space clears selection");
const before = await p.$eval("canvas", (c: HTMLCanvasElement) => c.toDataURL());
await p.mouse.move(EMPTY.x, EMPTY.y);
await p.mouse.down();
await p.mouse.move(EMPTY.x - 50, EMPTY.y - 50, { steps: 4 });
await p.mouse.up();
check((await p.$eval("canvas", (c: HTMLCanvasElement) => c.toDataURL())) !== before && (await state()).changes === s3.changes, "empty drag pans, model untouched");

// inline edit: double-click opens an overlay over the item, Enter commits, Esc cancels
const info = () =>
  p.evaluate(() => {
    const s = (window as any).editor.session;
    const ta = document.querySelector("textarea");
    const r = ta?.getBoundingClientRect();
    return { name: s.element.items[0].name, n: s.element.items.length, sel: s.selection.length, changes: (window as any).changes, ta: ta !== null, taRect: r && { x: r.x, y: r.y, w: r.width, h: r.height }, taValue: ta?.value, focus: document.activeElement?.tagName };
  });
// Drag and redo left item 0 on top of another one; edit it where it started.
await p.keyboard.press("Control+z");
await p.waitForTimeout(100);
const n0 = await info();
const pe = await at();
await p.mouse.dblclick(pe.x, pe.y);
await p.waitForTimeout(100);
const e1 = await info();
check(e1.ta && e1.taValue === n0.name && e1.focus === "TEXTAREA", `double-click opens the overlay with the name "${e1.taValue}"`);
check(e1.taRect !== null && e1.taRect!.x <= pe.x && pe.x <= e1.taRect!.x + e1.taRect!.w && e1.taRect!.y <= pe.y && pe.y <= e1.taRect!.y + e1.taRect!.h, "overlay covers the item");
await p.screenshot({ path: `${out}/6-editing.png` });
await p.keyboard.type("Renamed");
await p.keyboard.press("Enter");
await p.waitForTimeout(100);
const e2 = await info();
check(!e2.ta && e2.name === "Renamed" && e2.changes === n0.changes + 1, `Enter commits (${e2.name}), onChange +1`);
check((await p.evaluate(() => (window as any).lastText)).includes("Renamed"), "onChange text carries the new name");
await p.screenshot({ path: `${out}/7-renamed.png` });
await p.keyboard.press("Control+z");
await p.waitForTimeout(100);
check((await info()).name === n0.name, "ctrl+z restores the name");

const pe2 = await at();
await p.mouse.dblclick(pe2.x, pe2.y);
await p.keyboard.type("discarded");
await p.keyboard.press("Escape");
await p.waitForTimeout(100);
const e3 = await info();
check(!e3.ta && e3.name === n0.name && e3.changes === n0.changes + 2, "Esc cancels, no onChange");

// double-click on empty space still fits the view
await p.mouse.move(EMPTY.x, EMPTY.y);
await p.mouse.down();
await p.mouse.move(EMPTY.x - 50, EMPTY.y - 50, { steps: 4 });
await p.mouse.up();
const panned = await p.$eval("canvas", (c: HTMLCanvasElement) => c.toDataURL());
await p.mouse.dblclick(EMPTY.x + 10, EMPTY.y + 10);
await p.waitForTimeout(100);
check((await p.$eval("canvas", (c: HTMLCanvasElement) => c.toDataURL())) !== panned && !(await info()).ta, "double-click on empty space fits, no overlay");

// copy / paste / delete of an item with its link
const pick = () =>
  p.evaluate(() => {
    const s = (window as any).editor.session;
    const link = s.element.items.find((i: any) => i.kind === "Link");
    s.selection = [link.source, link.target, link];
    s.opts.onRedraw();
  });
await pick();
await p.screenshot({ path: `${out}/8-selected-link.png` });
const c0 = await info();
await p.keyboard.press("ControlOrMeta+c");
await p.keyboard.press("ControlOrMeta+v");
await p.waitForTimeout(100);
const c1 = await info();
check(c1.n === c0.n + 3 && c1.sel === 3 && c1.changes === c0.changes + 1, `copy+paste adds 3 items (${c0.n} -> ${c1.n}), they are selected, onChange +1`);
const pasted = await p.evaluate(() => {
  const s = (window as any).editor.session;
  const l = s.selection.find((i: any) => i.kind === "Link");
  return l.source === s.selection[0] && l.target === s.selection[1] && s.scene.drawables.has(l);
});
check(pasted, "pasted link joins the pasted items");
await p.screenshot({ path: `${out}/9-pasted.png` });
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(100);
check((await info()).n === c0.n, "undo removes the paste in one step");
await pick();
await p.keyboard.press("ControlOrMeta+x");
await p.waitForTimeout(100);
const x1 = await info();
check(x1.n < c0.n && x1.sel === 0, `cut removes the selection and its links (${c0.n} -> ${x1.n})`);
await p.keyboard.press("ControlOrMeta+v");
await p.waitForTimeout(100);
check((await info()).n === x1.n + 3, "paste brings the three items back");
await p.keyboard.press("ControlOrMeta+z");
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(100);
check((await info()).n === c0.n, "two undos restore the document");
await pick();
await p.keyboard.press("Backspace");
await p.waitForTimeout(100);
check((await info()).n === x1.n, "Backspace deletes the selection");
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(100);
check((await info()).n === c0.n, "undo restores the deleted items");
await pick();
await p.keyboard.press("ControlOrMeta+d");
await p.waitForTimeout(100);
const d1 = await info();
check(d1.n > c0.n && d1.sel === 3, `cmd+d duplicates (${c0.n} -> ${d1.n})`);
await p.screenshot({ path: `${out}/10-duplicated.png` });
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(100);
check((await info()).n === c0.n, "undo removes the duplicate");

// properties panel
const panel = () =>
  p.evaluate(() => {
    const s = (window as any).editor.session;
    const content = document.querySelector("#props .cm-content") as HTMLElement;
    const spans = (cls: string) => [...content.querySelectorAll(cls)].map((e) => ({ text: e.textContent, color: getComputedStyle(e).color }));
    return {
      text: content.innerText,
      symbols: spans(".tn-symbol"),
      strings: spans(".tn-string"),
      values: [...content.querySelectorAll(".tn-value")].map((e) => e.textContent),
      lint: content.querySelectorAll(".cm-lintRange-error").length,
      model: s.text() as string,
      changes: (window as any).changes as number,
      editable: content.getAttribute("contenteditable"),
    };
  });
const redPixels = () => p.$eval("canvas", (c: HTMLCanvasElement) => { const o = document.createElement("canvas"); o.width = c.width; o.height = c.height; const x = o.getContext("2d", { willReadFrequently: true })!; x.drawImage(c, 0, 0); const d = x.getImageData(0, 0, c.width, c.height).data; let n = 0; for (let i = 0; i < d.length; i += 4) if (d[i]! > 200 && d[i + 1]! < 60 && d[i + 2]! < 60 && d[i + 3]! > 200) n++; return n; });
const focusCanvas = () => p.evaluate(() => (document.querySelector("#editor > div") as HTMLElement).focus());
await p.evaluate(() => { const s = (window as any).editor.session; s.selection = [s.element.items[0]]; s.opts.onRedraw(); });
await p.waitForTimeout(200);
const k0 = await panel();
check(k0.symbols.length > 0 && k0.text.includes("pos"), `panel shows the item properties (${k0.symbols.length} symbols)`);
check(k0.symbols.every((x) => x.color === "rgb(129, 95, 3)") && k0.strings.every((x) => x.color === "rgb(28, 0, 207)"), "symbols and strings use the TennColors light colors");
await p.screenshot({ path: `${out}/11-panel-item.png` });
const red0 = await redPixels();
await p.click("#props .cm-content");
await p.keyboard.press("ControlOrMeta+End");
await p.keyboard.type("\ncolor red");
const k1 = await panel();
check(k1.changes === k0.changes, "typing alone does not apply");
await p.waitForTimeout(600);
const k2 = await panel();
check(k2.changes === k0.changes + 1 && k2.model.includes("color red"), `applied after the pause, onChange +1 (${k2.changes - k0.changes})`);
check((await redPixels()) > red0, `item is red on the canvas (${red0} -> ${await redPixels()} red pixels)`);
await p.screenshot({ path: `${out}/12-panel-red.png` });
await focusCanvas();
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(200);
const k3 = await panel();
check(k3.model.split("color red").length === k0.model.split("color red").length && !k3.text.includes("color red") && (await redPixels()) === red0, "undo removes color from the model, the panel and the canvas");
await p.click("#props .cm-content");
await p.keyboard.press("ControlOrMeta+End");
await p.keyboard.type("\nfoo $(6 * 7)");
await p.waitForTimeout(600);
const k4 = await panel();
check(k4.values.includes("42"), `expression value at line end (${JSON.stringify(k4.values)})`);
await p.screenshot({ path: `${out}/13-panel-value.png` });
await p.keyboard.type("\nbad {");
await p.waitForTimeout(700);
const k5 = await panel();
check(k5.lint > 0 && k5.changes === k4.changes, `syntax error is underlined (${k5.lint}), nothing applied`);
await p.screenshot({ path: `${out}/14-panel-error.png` });
await p.keyboard.press("Tab");
check((await panel()).text.includes("bad {    "), "Tab inserts 4 spaces");
await p.keyboard.press("ControlOrMeta+a");
await p.keyboard.press("Backspace");
await p.keyboard.type("{");
await p.keyboard.press("Enter");
await p.keyboard.type("a");
check((await panel()).text.includes("{\n    a"), "Enter after { indents");
await focusCanvas();
await p.keyboard.press("Escape");
await p.evaluate(() => { const s = (window as any).editor.session; s.selection = []; s.opts.onRedraw(); });
await p.waitForTimeout(200);
const k6 = await panel();
check(k6.text.includes('name "') && !k6.text.includes("{\n    a"), "no selection: panel shows the element properties, unsent edit dropped");
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(100);
check((await info()).n === c0.n, "(panel edits applied above are undone separately)") ;

// outline: switch, rename, add, delete, drag
const ol = () =>
  p.evaluate(() => {
    const ed = (window as any).editor;
    const root = ed.session.root;
    const count = (e: any): number => 1 + e.elements.reduce((n: number, c: any) => n + count(c), 0);
    return {
      el: ed.session.element.name as string,
      total: root.elements.reduce((n: number, c: any) => n + count(c), 0) as number,
      rows: document.querySelectorAll("#outline .outline-tree .row").length,
      sel: document.querySelector("#outline .outline-tree .row.sel .name")?.textContent,
      props: document.querySelector("#props .cm-content")?.textContent ?? "",
      changes: (window as any).changes as number,
      sels: ed.session.selection.map((i: any) => i.name) as string[],
    };
  });
const rowOf = (name: string) => p.locator(`#outline .outline-tree .row:has(> .name:text-is("${name}"))`);
const inkNow = () => p.$eval("canvas", (c: HTMLCanvasElement) => { const o = document.createElement("canvas"); o.width = c.width; o.height = c.height; const x = o.getContext("2d", { willReadFrequently: true })!; x.drawImage(c, 0, 0); return x.getImageData(0, 0, c.width, c.height).data.some((v, i) => i % 4 === 3 && v !== 0); });
await focusCanvas();
const o0 = await ol();
check(o0.rows > 10 && o0.sel === o0.el, `outline lists the elements (${o0.rows} rows), the current one is highlighted ("${o0.sel}")`);
await rowOf("General info").click();
await p.waitForTimeout(200);
const o1 = await ol();
check(o1.el === "General info" && o1.sel === "General info" && (await inkNow()), `click on an outline row shows that element ("${o1.el}")`);
check(o1.props.includes("General info") && o1.changes === o0.changes, "properties panel follows the element, no onChange");
await p.screenshot({ path: `${out}/15-outline-switched.png` });

await rowOf("General info").locator("> .name").dblclick();
await p.keyboard.press("ControlOrMeta+a");
await p.keyboard.type("Info X");
await p.keyboard.press("Enter");
await p.waitForTimeout(200);
const o2 = await ol();
check(o2.el === "Info X" && o2.sel === "Info X" && o2.changes === o1.changes + 1, `rename in the outline: "${o2.el}", onChange +1`);
check((await p.evaluate(() => (window as any).lastText)).includes('element "Info X"'), "onChange text carries the new element name");
await p.screenshot({ path: `${out}/16-outline-renamed.png` });
await rowOf("Basic steps").click();
await p.waitForTimeout(100);
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(200);
const o3 = await ol();
check(o3.el === "Basic steps" && (await rowOf("General info").count()) === 1 && o3.changes === o2.changes + 1, "ctrl+z after switching elements undoes the rename (history kept)");

await rowOf("Init screens").click();
await p.click('#outline button[title^="New element inside"]');
await p.waitForTimeout(200);
const o4 = await ol();
check(o4.el.startsWith("Unnamed element") && o4.total === o3.total + 1 && o4.changes === o3.changes + 1, `+ adds an element inside the current one and shows it ("${o4.el}")`);
await p.screenshot({ path: `${out}/17-outline-added.png` });
await p.keyboard.press("Escape");
await p.evaluate(() => (document.querySelector("#editor > div") as HTMLElement).focus());
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(200);
const o5 = await ol();
check(o5.el === "Init screens" && o5.total === o3.total, `undo of the add removes it, the editor falls back to the parent ("${o5.el}")`);

await rowOf("Simple").click();
await p.click('#outline button[title^="Delete"]');
await p.waitForTimeout(200);
const o6 = await ol();
check(o6.total === o3.total - 1 && o6.el === "Init screens" && (await rowOf("Simple").count()) === 0, "- deletes the element, the parent is shown");
await p.screenshot({ path: `${out}/18-outline-deleted.png` });
await focusCanvas();
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(200);
check((await ol()).total === o3.total && (await rowOf("Simple").count()) === 1, "undo brings the element back");

await rowOf("Simple").dragTo(rowOf("Executions"));
await p.waitForTimeout(200);
const parentOf = (name: string) => p.evaluate((n) => { const f = (e: any): any => e.elements.map((c: any) => (c.name === n ? c.parent.name : f(c))).find((x: any) => x); return f((window as any).editor.session.root); }, name);
check((await parentOf("Simple")) === "Executions" && (await ol()).changes === o6.changes + 2, "dragging a row onto another moves the element in one step");
await p.screenshot({ path: `${out}/19-outline-moved.png` });
await focusCanvas();
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(200);
check((await parentOf("Simple")) === "Init screens", "undo moves it back");
const indexOf = (name: string) => p.evaluate((n) => { const f = (e: any): any => e.elements.map((c: any, i: number) => (c.name === n ? [c.parent.name, i] : f(c))).find((x: any) => x); return f((window as any).editor.session.root); }, name);
const ex = await indexOf("Executions");
await rowOf("Simple").dragTo(rowOf("Executions"), { targetPosition: { x: 60, y: 2 } });
await p.waitForTimeout(200);
const sim = await indexOf("Simple");
check(sim[0] === ex[0] && sim[1] === ex[1], `dropping on the top edge of a row puts the element before it (${JSON.stringify(ex)} -> ${JSON.stringify(sim)})`);
await focusCanvas();
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(200);
check((await parentOf("Simple")) === "Init screens", "undo moves it back (index drop)");

// outline clipboard: copy + paste into another element, cut, one undo step each
const oc0 = await ol();
await rowOf("Simple").click();
await p.keyboard.press("ControlOrMeta+c");
await rowOf("Executions").click();
await p.keyboard.press("ControlOrMeta+v");
await p.waitForTimeout(200);
const oc1 = await ol();
check(oc1.total === oc0.total + 1 && oc1.changes === oc0.changes + 1 && (await parentOf("Simple")) === "Init screens", "outline Cmd+C / Cmd+V adds a copy of the element into the selected one");
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(200);
check((await ol()).total === oc0.total, "undo removes the pasted element in one step");
await rowOf("Simple").click();
await p.keyboard.press("ControlOrMeta+x");
await p.waitForTimeout(200);
const oc2 = await ol();
check(oc2.total === oc0.total - 1 && oc2.changes === oc0.changes + 3 && (await rowOf("Simple").count()) === 0, "outline Cmd+X removes the element");
await p.keyboard.press("ControlOrMeta+v");
await p.waitForTimeout(200);
check((await ol()).total === oc0.total && (await rowOf("Simple").count()) === 1, "Cmd+V brings the cut element back");
await focusCanvas();
for (let i = 0; i < 2; i++) await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(200);
check((await ol()).total === oc0.total && (await parentOf("Simple")) === "Init screens", "undo twice restores the original tree");

{
// outline keyboard: arrows walk the visible rows, left / right fold, Tab adds an item and focuses the canvas
const tree = () => p.locator("#outline .outline-tree");
const nameOf = () => p.evaluate(() => (window as any).editor.session.element.name as string);
await tree().focus();
await rowOf("Documentation").click();
const k0 = await nameOf();
await p.keyboard.press("ArrowDown");
const k1 = await nameOf();
await p.keyboard.press("ArrowUp");
check(k1 !== k0 && (await nameOf()) === k0, `ArrowDown moves to the next row ("${k0}" -> "${k1}"), ArrowUp back`);
await p.keyboard.press("ArrowLeft");
const folded = await p.evaluate(() => document.querySelector("#outline .outline-tree .row.sel + .children")?.classList.contains("hidden") ?? null);
const rowsFolded = (await ol()).rows;
check(folded === true, "ArrowLeft collapses the current element");
await p.keyboard.press("ArrowRight");
await p.keyboard.press("ArrowRight");
check((await nameOf()) !== k0 && (await p.locator("#outline .outline-tree .row:visible").count()) > 0, `ArrowRight expands, then goes to the first child ("${await nameOf()}")`);
await p.keyboard.press("ArrowLeft");
check((await nameOf()) === k0, "ArrowLeft on a leaf goes to the parent");
const items0 = await p.evaluate(() => (window as any).editor.session.element.items.length as number);
await p.keyboard.press("Tab");
const tab = await p.evaluate(() => ({ items: (window as any).editor.session.element.items.length as number, focus: document.activeElement === document.querySelector("#editor > div"), sel: (window as any).editor.session.selection.length as number }));
check(tab.items === items0 + 1 && tab.focus && tab.sel === 1, "Tab adds an item to the diagram, selects it and focuses the canvas");
await p.keyboard.press("ControlOrMeta+z");
await p.screenshot({ path: `${out}/20-outline-keys.png` });
await rowOf("Basic steps").click();

// title bar: zoom steps, reset, add / remove item
const zoom = () => p.evaluate(() => ({ label: document.querySelector(".tn-zoom")!.textContent, k: (window as any).editor.session && document.querySelector(".tn-zoom")!.textContent }));
const lab = async () => (await zoom()).label;
await p.click('.tn-bar button[title="Reset zoom"]');
await p.waitForTimeout(100);
check((await lab()) === "100%", "100% button resets the zoom");
await p.click('.tn-bar button[title="Zoom in"]');
await p.waitForTimeout(100);
check((await lab()) === "133%", `zoom in: 100% -> ${await lab()} (1 / 0.75)`);
await p.click('.tn-bar button[title="Zoom out"]');
await p.click('.tn-bar button[title="Zoom out"]');
await p.waitForTimeout(100);
check((await lab()) === "75%", `zoom out twice: ${await lab()} (0.75 per step)`);
await p.screenshot({ path: `${out}/22-toolbar.png` });
const n0 = await p.evaluate(() => (window as any).editor.session.element.items.length as number);
await p.click('.tn-bar .tn-seg button[title="New item"]');
check((await p.evaluate(() => (window as any).editor.session.element.items.length as number)) === n0 + 1, "toolbar + adds an item");
await p.click('.tn-bar .tn-seg button[title="Delete selection"]');
check((await p.evaluate(() => (window as any).editor.session.element.items.length as number)) === n0, "toolbar - removes the selected item");
await p.click('.tn-bar button[title="Share"]');
const share = await p.evaluate(() => [...document.querySelectorAll(".tn-menu.root > .tn-mi")].map((e) => `${e.firstChild!.textContent}:${e.querySelector("svg") !== null}`));
check(share.length === 9 && share.every((l) => l.endsWith(":true")), `share menu has icons: ${share.join(", ")}`);
await p.screenshot({ path: `${out}/23-share-menu.png` });
await p.keyboard.press("Escape");
// Each export lands in a download with the expected name and content.
await p.context().grantPermissions(["clipboard-read", "clipboard-write"]);
const exported = async (label: string) => {
  await p.click('.tn-bar button[title="Share"]');
  const [dl] = await Promise.all([p.waitForEvent("download"), p.click(`.tn-menu.root >> text="${label}"`)]);
  const file = `${out}/export-${dl.suggestedFilename()}`;
  await dl.saveAs(file);
  return { name: dl.suggestedFilename() as string, text: readFileSync(file, "utf8"), file };
};
const ename = await p.evaluate(() => (window as any).editor.session.element.name as string);
const html = await exported("Export as HTML");
check(html.name === `${ename}.html` && /^<html>[\s\S]*src="data:image\/png;base64,[A-Za-z0-9+/=]{100,}"/.test(html.text), `Export as HTML: ${html.name}`);
const json = await exported("Export as JSON");
const parsed = JSON.parse(json.text);
check(json.name === `${ename}.json` && parsed.name === ename && Array.isArray(parsed.items) && json.text.includes('"name" : '), `Export as JSON: ${json.name}, ${parsed.items.length} items`);
// Print: window.print is stubbed; the page then holds the element as SVG in the print host, in the same family as the canvas.
await p.evaluate(() => { (window as any).prints = 0; window.print = () => void ((window as any).prints += 1); });
const printed = () => p.evaluate(() => { const h = document.getElementById("tn-print"); return { n: (window as any).prints as number, svg: h?.querySelector("svg")?.outerHTML ?? "", css: h?.querySelector("style")?.textContent ?? "" }; });
await p.click('.tn-bar button[title="Share"]');
await p.click('.tn-menu.root >> text="Export as PDF"');
const pdf1 = await printed();
check(pdf1.n === 1 && /^<svg[^>]*viewBox/.test(pdf1.svg) && pdf1.svg.includes("<text") && pdf1.css.includes("size:"), `Export as PDF prints the SVG: ${pdf1.svg.length} B`);
check(!pdf1.svg.includes("fill=\"#e7e9eb\""), "print SVG has no background fill");
await p.mouse.click(EMPTY.x, EMPTY.y);
await p.keyboard.press("Control+p");
const pdf2 = await printed();
check(pdf2.n === 2 && !pdf2.css.includes("size:") && pdf2.svg.length > 0, "Cmd+P prints with paper margins");
const pdfBytes: Buffer = await p.pdf({ preferCSSPageSize: false, printBackground: true });
writeFileSync(`${out}/print.pdf`, pdfBytes);
check(pdfBytes.toString("latin1").match(/\/Type \/Page\b/g)?.length === 1 && (await p.$eval("#tn-print", (e: HTMLElement) => getComputedStyle(e).display)) === "none", "print media: one page, host hidden on screen");
const inter = await exported("Export as interactive HTML");
check(inter.name === `${ename}.html` && inter.text.startsWith("<!doctype html>") && inter.text.includes('data-encoding="base64"') && inter.text.length > 800_000, `Export as interactive HTML: ${inter.text.length} B`);
const page = await browser.newPage({ viewport: { width: 1000, height: 700 } });
await page.goto(`file://${inter.file}`);
await page.waitForSelector("canvas");
await page.waitForTimeout(500);
await page.screenshot({ path: `${out}/interactive-html.png` });
check(await page.$eval("canvas", (c: HTMLCanvasElement) => c.getContext("2d")!.getImageData(0, 0, c.width, c.height).data.some((v, i) => i % 4 === 3 && v !== 0)), "interactive HTML renders a diagram");
await page.close();
await p.click('.tn-bar button[title="Share"]');
await p.click('.tn-menu.root >> text="Copy as JSON"');
await p.waitForTimeout(300);
check((await p.evaluate(() => navigator.clipboard.readText())).includes('"items"'), "Copy as JSON puts JSON on the clipboard");
await p.click('.tn-bar button[title="Share"]');
await p.click('.tn-menu.root >> text="Copy as HTML"');
await p.waitForTimeout(500); // PNG render before the clipboard write
check((await p.evaluate(async () => (await (await navigator.clipboard.read())[0]!.getType("text/html")).text())).includes("data:image/png;base64,"), "Copy as HTML puts text/html on the clipboard");

// off-screen indicators: pan everything out of view, dots appear at the edge
const dots = () => p.$eval("canvas", (c: HTMLCanvasElement) => { const o = document.createElement("canvas"); o.width = c.width; o.height = c.height; const x = o.getContext("2d", { willReadFrequently: true })!; x.drawImage(c, 0, 0); const w = c.width; const h = c.height; let n = 0; for (const [x0, y0, ww, hh] of [[0, 0, 40, h], [w - 40, 0, 40, h], [0, 0, w, 40], [0, h - 40, w, 40]] as number[][]) { const d = x.getImageData(x0!, y0!, ww!, hh!).data; for (let i = 3; i < d.length; i += 4) if (d[i] !== 0) n++; } return n; });
await p.click('.tn-bar button[title="Reset zoom"]');
await p.mouse.move(EMPTY.x, EMPTY.y);
const dots0 = await dots();
for (let i = 0; i < 6; i++) await p.mouse.wheel(-1500, 0);
await p.waitForTimeout(300);
const dots1 = await dots();
check(dots1 > 0 && dots1 < dots0, `panning items off-screen leaves only indicators at the edge (${dots0} -> ${dots1} edge pixels)`);
await p.screenshot({ path: `${out}/24-indicators.png` });
await p.click('.tn-bar button[title="Reset zoom"]');
await p.evaluate(() => {
  const s = (window as any).editor.session;
  const find = (e: any): any => e.items.find((i: any) => i.name === "Main Window") ?? e.elements.map(find).find((x: any) => x);
  s.reveal(find(s.root));
});
await p.waitForTimeout(200);
}

// properties panel: long lines scroll (no wrap)
const sc0 = await p.evaluate(() => { const e = document.querySelector("#props .cm-scroller") as HTMLElement; const before = e.scrollLeft; e.scrollLeft = 100; return { sw: e.scrollWidth, cw: e.clientWidth, ox: getComputedStyle(e).overflowX, moved: e.scrollLeft - before }; });
check(sc0.sw > sc0.cw && sc0.ox === "auto" && sc0.moved > 0, `panel scrolls horizontally (${sc0.sw} > ${sc0.cw}, moved ${sc0.moved})`);
await p.screenshot({ path: `${out}/30-panel-scroll.png` });

// images: paste of a PNG file, "Attach image" through the file chooser
const imgItems = () => p.evaluate(() => { const s = (window as any).editor.session; return { n: s.element.items.length, sel: s.selection.length, withImage: s.element.items.filter((i: any) => i.properties.get("image") !== null).length, changes: (window as any).changes as number }; });
const pasteBlob = (type: string) =>
  p.evaluate(async (t) => {
    const c = document.createElement("canvas");
    [c.width, c.height] = [40, 30];
    const g = c.getContext("2d")!;
    g.fillStyle = "#e0245e";
    g.fillRect(0, 0, 40, 30);
    const blob: Blob = await new Promise((r) => c.toBlob((b) => r(b!), t));
    const dt = new DataTransfer();
    dt.items.add(new File([blob], t === "image/png" ? "shot.png" : "shot.jpg", { type: t }));
    document.querySelector("canvas")!.parentElement!.dispatchEvent(new ClipboardEvent("paste", { clipboardData: dt, bubbles: true, cancelable: true }));
  }, type);
await p.evaluate(() => { const s = (window as any).editor.session; s.selection = []; });
const i0 = await imgItems();
await pasteBlob("image/png");
await p.waitForFunction(() => (window as any).editor.session.text().includes("shot.png"), null, { timeout: 5000 }).catch(() => {});
const i1 = await imgItems();
check(i1.n === i0.n + 1 && i1.sel === 1 && i1.withImage === i0.withImage + 1 && i1.changes === i0.changes + 1, "pasted PNG: a new item with the image, selected, onChange +1");
const stored = await p.evaluate(() => { const s = (window as any).editor.session; const t: string = s.text(); return { named: t.includes('image "shot.png"'), title: t.includes("@(shot.png|96)") }; });
check(stored.named && stored.title, "pasted item stores image name and the title with the image");
await p.screenshot({ path: `${out}/31-pasted-image.png` });
await p.evaluate(() => (window as any).editor.undo());
await p.waitForTimeout(100);
check((await imgItems()).n === i0.n, "undo removes the pasted image item");
await pasteBlob("image/jpeg");
await p.waitForFunction(() => (window as any).editor.session.text().includes("shot.jpg"), null, { timeout: 5000 }).catch(() => {});
const jpg = await p.evaluate(() => { const t: string = (window as any).editor.session.text(); return t.includes('image "shot.jpg"') && t.includes("iVBORw0KGgo"); });
check(jpg, "pasted JPEG is stored as PNG");
await p.evaluate(() => (window as any).editor.undo());
await p.waitForTimeout(100);
await p.evaluate(() => { const s = (window as any).editor.session; s.selection = [s.element.items.find((i: any) => i.name === "Diagram area")]; s.opts.onRedraw(); });
const a0 = await imgItems();
{
  const pt = await p.evaluate(() => { const e = (window as any).editor; return e.screenOf(e.session.selection[0]); });
  await p.mouse.click(pt.x, pt.y, { button: "right" });
  await p.waitForTimeout(100);
  const png = Buffer.from(await p.evaluate(() => { const c = document.createElement("canvas"); c.width = c.height = 8; return c.toDataURL("image/png").split(",")[1]!; }), "base64");
  const [chooser] = await Promise.all([p.waitForEvent("filechooser"), p.click('.tn-menu.root >> text="Attach image"')]);
  await chooser.setFiles({ name: "dot.png", mimeType: "image/png", buffer: png });
  await p.waitForFunction(() => (window as any).editor.session.text().includes("dot.png"), null, { timeout: 5000 }).catch(() => {});
}
const a1 = await imgItems();
check(a1.n === a0.n && a1.withImage === a0.withImage + 1 && a1.changes === a0.changes + 1, "Attach image: the selected item gets an image, onChange +1");
await p.evaluate(() => (window as any).editor.undo());
await p.waitForTimeout(100);
check((await imgItems()).withImage === a0.withImage, "undo removes the attached image");

// styles context menu
const st = () => p.evaluate(() => { const s = (window as any).editor.session; const it = s.selection[0]; return { model: s.text() as string, changes: (window as any).changes as number, styles: s.styleNames() as string[], has: it ? s.propsText(it) as string : "" }; });
const rightClick = async (at?: { x: number; y: number }) => {
  // "Diagram area" is a plain box; the screenshot item "Main Window" is not hit at its centre.
  const pt = at ?? (await p.evaluate(() => { const e = (window as any).editor; return e.screenOf(e.session.element.items.find((i: any) => i.name === "Diagram area")); }));
  await p.mouse.click(pt.x, pt.y, { button: "right" });
  await p.waitForTimeout(100);
};
const menuLabels = () => p.evaluate(() => [...document.querySelectorAll(".tn-menu.root > .tn-mi")].map((e) => e.firstChild!.textContent));
// Hover the parents, click the last label; only items of open menus count.
const pickMenu = async (...path: string[]) => {
  for (const [i, label] of path.entries()) {
    const box = await p.evaluate((l) => {
      const el = [...document.querySelectorAll<HTMLElement>(".tn-mi")].find((e) => e.firstChild?.textContent === l && e.offsetParent !== null);
      const r = el?.getBoundingClientRect();
      return r && { x: r.x + r.width / 2, y: r.y + r.height / 2 };
    }, label);
    if (box === undefined || box === null) throw new Error(`menu item "${label}" is not visible`);
    await p.mouse.move(box.x, box.y);
    if (i === path.length - 1) await p.mouse.down(), await p.mouse.up();
    await p.waitForTimeout(50);
  }
  await p.waitForTimeout(100);
};
await p.evaluate(() => { const s = (window as any).editor.session; s.selection = []; s.opts.onRedraw(); });
await rightClick();
const t0 = await st();
check((await p.evaluate(() => (window as any).editor.session.selection.map((i: any) => i.name))).join() === "Diagram area", "right click selects the item under the cursor");
check(JSON.stringify(await menuLabels()) === JSON.stringify(["New item", "New linked item", "Linked styled item", "Style", "Quick Style", "Duplicate", "Attach image", "Order", "Export text as html", "Delete"]), `context menu on an item: ${JSON.stringify(await menuLabels())}`);
await p.hover(".tn-menu.root > .tn-mi:nth-child(7)");
await p.screenshot({ path: `${out}/31-styles.png` });
await pickMenu("Export text as html");
await p.waitForTimeout(200);
const itemHtml = await p.evaluate(async () => (await (await navigator.clipboard.read())[0]!.getType("text/html")).text());
check(itemHtml.startsWith("<div>") && itemHtml.includes("Diagram area"), `Export text as html copies the item text: ${itemHtml.slice(0, 60)}`);
await rightClick();
await pickMenu("Quick Style", "Color", "🔴red");
const t1 = await st();
check(t1.has.includes("color red") && t1.changes === t0.changes + 1 && t1.model.includes("color red") && (await p.locator(".tn-menu").count()) === 0, "quick style: color red in the model, onChange +1, menu closed");
await rightClick();
await pickMenu("Quick Style", "Display", "● circle");
check((await st()).has.includes("display circle"), "quick style: display circle");
await p.keyboard.press("ControlOrMeta+z");
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(100);
const t2 = await st();
check(!t2.has.includes("color red") && !t2.has.includes("display circle") && t2.changes === t0.changes + 4, "undo x2 restores the item");
await rightClick();
await pickMenu("Style", "Define new style");
const t3 = await st();
check(t3.styles.length === t0.styles.length + 1 && t3.changes === t2.changes + 1, `new style defined (${t3.styles.join(",")})`);
const newName = t3.styles[t3.styles.length - 1]!;
await rightClick();
await pickMenu("Style", newName);
const t4 = await st();
check(t4.has.includes(`use-style ${newName}`) && t4.changes === t3.changes + 1, "style applied to the selection, one onChange");
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(100);
check(!(await st()).has.includes("use-style"), "undo removes use-style");
await rightClick();
await p.keyboard.press("Escape");
check((await p.locator(".tn-menu").count()) === 0, "Esc closes the menu");
await rightClick(EMPTY);
check(JSON.stringify(await menuLabels()) === JSON.stringify(["New item", "Test layout", "Style", "Global Styles"]) && (await p.evaluate(() => (window as any).editor.session.selection.length)) === 0, `right click on empty space: selection cleared, ${JSON.stringify(await menuLabels())}`);
await pickMenu("Global Styles", "Enable shadows");
check((await st()).changes === t4.changes + 2 && (await st()).model.includes("shadow -5 -5 5"), "Enable shadows applies in one step");

// ctrl-drag: preview line, link on drop, no context menu, one undo step; empty drop adds nothing
const linkCount = () => p.evaluate(() => (window as any).editor.session.element.items.filter((i: any) => i.kind === "Link").length);
const screenOf = (name: string) => p.evaluate((n) => { const e = (window as any).editor; return e.screenOf(e.session.element.items.find((i: any) => i.name === n)); }, name);
const [from, to] = [await screenOf("Diagram area"), await screenOf("Outline")];
const l0 = await linkCount();
const cl0 = (await st()).changes;
await p.keyboard.down("Control");
await p.mouse.move(from.x, from.y);
await p.mouse.down();
await p.mouse.move((from.x + to.x) / 2, (from.y + to.y) / 2, { steps: 4 });
check(await p.evaluate(() => (window as any).editor.session.scene.lineToDrawable !== null), "ctrl-drag: preview line is drawn");
await p.mouse.move(to.x, to.y, { steps: 4 });
await p.mouse.up();
await p.keyboard.up("Control");
await p.waitForTimeout(100);
check((await linkCount()) === l0 + 1 && (await st()).changes === cl0 + 1, "ctrl-drag onto an item adds one link, one onChange");
check((await p.locator(".tn-menu").count()) === 0, "ctrl-click does not open the context menu");
await p.screenshot({ path: `${out}/32-ctrl-link.png` });
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(100);
check((await linkCount()) === l0, "undo removes the dragged link");
await p.keyboard.down("Control");
await p.mouse.move(from.x, from.y);
await p.mouse.down();
await p.mouse.move(EMPTY.x, EMPTY.y, { steps: 4 });
await p.mouse.up();
await p.keyboard.up("Control");
await p.waitForTimeout(100);
check((await linkCount()) === l0 && (await p.evaluate(() => (window as any).editor.session.scene.lineToDrawable)) === null, "ctrl-drag dropped on empty space adds nothing, preview gone");
await p.keyboard.press("ControlOrMeta+z");

// context menu actions and canvas keys
{
  const n = () => p.evaluate(() => (window as any).editor.session.element.items.length as number);
  // "Diagram area" is a plain box that is hit at its centre.
  const sess = <R>(f: string): Promise<R> => p.evaluate(`(() => { const s = window.editor.session; const T = () => s.element.items.find((i) => i.name === "Diagram area"); return ${f}; })()`) as Promise<R>;
  const first = () => sess<{ x: number; y: number }>("({ x: T().x, y: T().y })");
  const changes = () => p.evaluate(() => (window as any).changes as number);
  const selectFirst = () => sess<void>("(s.selection = [T()], s.opts.onRedraw())");
  const positions = () => sess<string>("JSON.stringify(s.element.items.map((i) => [i.x, i.y]))");
  await p.keyboard.press("Escape");
  await focusCanvas();
  await selectFirst();
  const n0 = await n();
  await rightClick();
  await pickMenu("New linked item");
  check((await n()) === n0 + 2 && (await sess<string>("s.selection[0].kind")) === "Item" && (await sess<string>("s.selection[0].name")).startsWith("Untitled"), "menu: New linked item adds an item and its link, selects the item");
  await p.keyboard.press("ControlOrMeta+z");
  check((await n()) === n0, "undo removes both in one step");
  await selectFirst();
  await rightClick();
  await pickMenu("Duplicate");
  check((await n()) > n0, "menu: Duplicate");
  await p.keyboard.press("ControlOrMeta+z");
  await selectFirst();
  await rightClick();
  await pickMenu("Delete");
  check((await n()) < n0, "menu: Delete");
  await p.keyboard.press("ControlOrMeta+z");
  await sess<void>("(s.selection = [T(), ...s.element.items.filter((i) => i.kind === 'Item' && i !== T()).slice(0, 2)], s.opts.onRedraw())");
  await rightClick();
  const labels3 = await menuLabels();
  check((await sess<number>("s.selection.length")) === 3 && labels3.includes("Align") && !labels3.includes("Order"), `several selected: Align, no Order (${labels3.join(", ")})`);
  await pickMenu("Align", "Leading Edges");
  check((await sess<number[]>("s.selection.map((i) => i.x)")).every((x, _, all) => x === all[0]), "menu: Align > Leading Edges");
  await p.keyboard.press("ControlOrMeta+z");
  await selectFirst();
  await rightClick();
  await pickMenu("Order", "Move Forward");
  check(await sess<boolean>("s.element.items.at(-1) === s.selection[0]"), "menu: Order > Move Forward");
  await p.keyboard.press("ControlOrMeta+z");
  await sess<void>("(s.selection = [], s.opts.onRedraw())");
  const l0 = await positions();
  await rightClick(EMPTY);
  await pickMenu("Test layout");
  check((await positions()) !== l0, "menu: Test layout moves the items");
  await p.keyboard.press("ControlOrMeta+z");
  check((await positions()) === l0, "undo restores the layout in one step");

  await focusCanvas();
  await selectFirst();
  const c0 = await changes();
  await p.keyboard.press("Tab");
  check((await n()) === n0 + 2 && (await changes()) === c0 + 1, "Tab adds a linked item in one step");
  await p.keyboard.press("ControlOrMeta+z");
  await selectFirst();
  await sess<void>('s.setQuickStyle("color", "red")');
  await p.keyboard.press("Alt+Tab");
  check((await sess<string>("s.propsText(s.selection[0])")).includes("color red"), "Option+Tab copies the style properties into the new item");
  await p.keyboard.press("ControlOrMeta+z");
  await p.keyboard.press("ControlOrMeta+z");
  await selectFirst();
  await p.keyboard.press("x");
  check((await n()) < n0, "x removes the selection");
  await p.keyboard.press("ControlOrMeta+z");
  check((await n()) === n0, "undo brings it back");
  await selectFirst();
  const a0 = await first();
  const c1 = await changes();
  await p.keyboard.press("ArrowRight");
  const a1 = await first();
  check(a1.x > a0.x && a1.x - a0.x <= 10 && a1.x % 5 === 0, `ArrowRight moves to the next grid line (${a0.x} -> ${a1.x})`);
  await p.keyboard.press("ArrowUp");
  check((await first()).y > a1.y && (await changes()) === c1 + 2, "ArrowUp moves up, one undo step per press");
  await p.keyboard.press("ControlOrMeta+z");
  await p.keyboard.press("ControlOrMeta+z");
  check(JSON.stringify(await first()) === JSON.stringify(a0), "two undos restore the position");
  const text0 = await sess<string>("s.propsText(s.selection[0])");
  await p.keyboard.press("Shift+ArrowRight");
  const w1 = await sess<string>("s.propsText(s.selection[0])");
  check(/width [\d.]+/.test(w1) && JSON.stringify(await first()) === JSON.stringify(a0), "Shift+ArrowRight resizes (width property), position stays");
  await p.keyboard.press("Meta+ArrowRight");
  check((await first()).x < a0.x && (await sess<string>("s.propsText(s.selection[0])")) !== w1, "Cmd+ArrowRight resizes from the centre (item shifts left)");
  await p.keyboard.press("ControlOrMeta+z");
  await p.keyboard.press("ControlOrMeta+z");
  check((await sess<string>("s.propsText(s.selection[0])")) === text0, "undo of both resizes");
  await p.keyboard.press("ControlOrMeta+a");
  check((await sess<number>("s.selection.length")) === n0, "Cmd+A selects everything");
  await p.keyboard.press("ControlOrMeta+Shift+a");
  check(await sess<boolean>("s.selection.length > 0 && s.selection.every((i) => i.kind === 'Item')"), "Cmd+Shift+A selects the items");
  await selectFirst();
  await p.keyboard.press("Alt+Enter");
  check(await p.evaluate(() => document.querySelector("textarea") !== null), "Option+Enter opens the value editor");
  await p.keyboard.type("42");
  await p.keyboard.press("Enter");
  check((await sess<string>("s.propsText(s.selection[0])")).includes("value 42"), "value 42 stored as a number");
  await p.keyboard.press("ControlOrMeta+z");
  await sess<void>("(s.selection = [], s.opts.onRedraw())");
}

// Quick style panel: above the single selected item, follows pan, segment menu applies a style
{
  const sess = <R>(f: string): Promise<R> => p.evaluate(`(() => { const s = window.editor.session; const T = () => s.element.items.find((i) => i.name === "Diagram area"); return ${f}; })()`) as Promise<R>;
  const pop = () => p.evaluate(() => { const r = document.querySelector(".tn-pop")?.getBoundingClientRect(); return r && { x: r.x, y: r.y, w: r.width, h: r.height, n: document.querySelectorAll(".tn-pop button").length }; });
  await sess<void>("(s.selection = [T()], s.opts.onRedraw())");
  await p.waitForTimeout(400);
  const q0 = await pop();
  const c = await p.evaluate(() => { const e = (window as any).editor; return e.screenOf(e.session.element.items.find((i: any) => i.name === "Diagram area")); });
  check(q0 !== undefined && q0!.n === 6 && q0!.y + q0!.h <= c.y, `quick panel: 6 segments above the selected item (${JSON.stringify(q0)})`);
  await p.screenshot({ path: `${out}/quick-panel.png` });
  await p.mouse.move(EMPTY.x, EMPTY.y);
  await p.mouse.down();
  await p.mouse.move(EMPTY.x - 40, EMPTY.y - 30, { steps: 4 });
  await p.mouse.up();
  const q1 = await pop();
  check(q1 === undefined, "quick panel: the empty click that starts a pan clears the selection and hides it");
  await sess<void>("(s.selection = [T()], s.opts.onRedraw())");
  await p.waitForTimeout(100);
  const q2 = await pop();
  const c2 = await p.evaluate(() => { const e = (window as any).editor; return e.screenOf(e.session.element.items.find((i: any) => i.name === "Diagram area")); });
  // Down, away from the top edge where the panel is clamped.
  await p.mouse.wheel(0, -40);
  await p.waitForTimeout(300);
  const q3 = await pop();
  const c3 = await p.evaluate(() => { const e = (window as any).editor; return e.screenOf(e.session.element.items.find((i: any) => i.name === "Diagram area")); });
  check(q2 !== undefined && q3 !== undefined && c3.y > c2.y && Math.abs(q3.y - q2.y - (c3.y - c2.y)) < 1.5, `quick panel follows the wheel pan (item ${c2.y} -> ${c3.y}, panel ${q2?.y} -> ${q3?.y})`);
  await p.click(".tn-pop button[title='Color']");
  check((await p.locator(".tn-menu.root > .tn-mi").count()) === 8, "quick panel: the colour segment opens its 8 colours");
  await p.screenshot({ path: `${out}/quick-panel-menu.png` });
  const cp0 = await p.evaluate(() => (window as any).changes as number);
  await p.locator(".tn-menu.root > .tn-mi", { hasText: "red" }).click();
  check((await sess<string>("s.propsText(T())")).includes("color red") && (await p.evaluate(() => (window as any).changes as number)) === cp0 + 1, "quick panel: a pick sets the property, onChange +1");
  await p.keyboard.press("ControlOrMeta+z");
  check(!(await sess<string>("s.propsText(T())")).includes("color red"), "quick panel: undo restores the item");
  await p.evaluate(() => (window as any).editor.setSettings({ quickPanel: false }));
  await p.waitForTimeout(100);
  check((await pop()) === undefined, "quick panel: the setting hides it live");
  await p.evaluate(() => (window as any).editor.setSettings({ quickPanel: true }));
  await p.waitForTimeout(100);
  check((await pop()) !== undefined, "quick panel: and shows it again");
  await p.evaluate(() => (window as any).editor.edit("name"));
  await p.waitForTimeout(100);
  check((await pop()) === undefined, "quick panel: hidden while the name is edited");
  await p.keyboard.press("Escape");
  await p.waitForTimeout(100);
  check((await pop()) !== undefined, "quick panel: back after the edit");
  await sess<void>("(s.selection = [], s.opts.onRedraw())");
  await p.waitForTimeout(100);
  check((await pop()) === undefined, "quick panel: hidden on an empty selection");
  await sess<void>("(s.selection = s.element.items.slice(0, 2), s.opts.onRedraw())");
  await p.waitForTimeout(100);
  check((await pop()) === undefined, "quick panel: hidden for several selected items");
  await sess<void>("(s.selection = [s.element.items.find((i) => i.kind === 'Link')], s.opts.onRedraw())");
  await p.waitForTimeout(100);
  check((await pop())?.n === 3, "quick panel: a link gets 3 segments (display, line style, line width)");
  await sess<void>("(s.selection = [], s.opts.onRedraw())");
}

// Goto Item (Cmd+R): popup over the canvas, typing filters, arrows select and centre, Enter closes
{
  const names = () => p.evaluate(() => [...document.querySelectorAll(".tn-search .tn-sr")].map((e) => e.textContent!));
  const cur = () =>
    p.evaluate(() => {
      const e = (window as any).editor;
      const s = e.session;
      const sel = s.selection[0];
      const r = document.querySelector("canvas")!.getBoundingClientRect();
      const c = sel === undefined ? { x: 0, y: 0 } : e.screenOf(sel);
      return { name: sel?.name as string | undefined, dx: c.x - (r.left + r.width / 2), dy: c.y - (r.top + r.height / 2) };
    });
  await p.mouse.click(EMPTY.x, EMPTY.y);
  await p.keyboard.press("ControlOrMeta+r");
  check((await p.locator(".tn-search input").count()) === 1, "Cmd+R opens the search popup (no page reload)");
  await p.keyboard.type("a");
  const list = await names();
  check(list.length > 1 && list.join() === [...list].sort().join(), `typing lists matches by name (${list.length})`);
  const first = (await cur()).name;
  await p.keyboard.press("ArrowDown");
  const second = await cur();
  check(second.name !== undefined && second.name !== first && list[1]!.startsWith(second.name), `ArrowDown selects the next result ("${first}" -> "${second.name}")`);
  check(Math.abs(second.dx) < 2 && second.dy > 10, `the item is centred horizontally and below the centre, as Swift's offset 120 (${second.dx.toFixed(1)}, ${second.dy.toFixed(1)})`);
  await p.screenshot({ path: `${out}/goto-item.png` });
  await p.keyboard.press("Enter");
  check((await p.locator(".tn-search").count()) === 0 && (await cur()).name === second.name, "Enter closes the popup, the item stays selected");
  await p.keyboard.press("ControlOrMeta+r");
  await p.keyboard.press("Escape");
  check((await p.locator(".tn-search").count()) === 0, "Esc closes the popup");
}

// Operation box (Space): select, Space, type, Enter changes the item; invalid input turns the field red; undo restores
{
  const props = () => p.evaluate(() => { const e = (window as any).editor; return e.session.propsText(e.session.selection[0]); });
  await p.mouse.click(EMPTY.x, EMPTY.y);
  await p.keyboard.press("Space");
  check((await p.locator(".tn-op").count()) === 0, "Space without a selection does nothing");
  await p.evaluate(() => { const e = (window as any).editor; e.session.select([e.session.element.items.find((i: any) => i.kind === "Item")]); });
  const before = await props();
  const n0 = (await state()).changes;
  await p.keyboard.press("Space");
  check((await p.locator(".tn-op input").count()) === 1, "Space opens the operation box");
  await p.keyboard.type("color {");
  await p.keyboard.press("Enter");
  check((await p.locator(".tn-op input.bad").count()) === 1 && (await props()) === before, "invalid input: red field, popup stays, item unchanged");
  await p.screenshot({ path: `${out}/operation-error.png` });
  await p.keyboard.press("ControlOrMeta+a");
  await p.keyboard.type("color red");
  await p.keyboard.press("Enter");
  check((await p.locator(".tn-op").count()) === 0 && /color red/.test(await props()), "Enter applies and closes");
  check((await state()).changes === n0 + 1, "onChange fired once");
  await p.keyboard.press("ControlOrMeta+z");
  check((await props()) === before, "undo restores the item");
  await p.keyboard.press("Space");
  await p.keyboard.press("Escape");
  check((await p.locator(".tn-op").count()) === 0, "Esc closes the box");
  await p.evaluate(() => (window as any).editor.session.select([]));
}

// readonly
await p.click("#ro");
await p.waitForFunction(() => document.getElementById("status")!.textContent === "readonly");
const ro0 = (await state()).changes;
const pr = await at();
await p.mouse.dblclick(pr.x, pr.y);
await p.keyboard.press("Enter");
const nRo = (await info()).n;
await p.keyboard.press("ControlOrMeta+v"); // the clipboard still holds the last copy
await p.keyboard.press("ControlOrMeta+d");
await p.waitForTimeout(100);
check((await panel()).editable === "false", "readonly: panel is not editable");
check(!(await info()).ta && (await info()).n === nRo, "readonly: no overlay, paste and duplicate do nothing");
await p.mouse.click(pr.x, pr.y);
await p.mouse.move(pr.x, pr.y);
await p.mouse.down();
await p.mouse.move(pr.x + 80, pr.y + 60, { steps: 4 });
await p.mouse.up();
const sr = await state();
await p.mouse.click(pr.x, pr.y, { button: "right" });
check((await p.locator(".tn-menu").count()) === 0, "readonly: no context menu");
check(sr.sel === 0 && sr.x === s0.x && sr.y === s0.y && sr.changes === ro0, "readonly: no selection, no move, no onChange");
await p.screenshot({ path: `${out}/5-readonly.png` });
check((await p.locator(".tn-pop").count()) === 0, "readonly: no quick panel");
check((await p.locator("#outline .outline-header button").count()) === 0, "readonly: no outline buttons");
const roChanges = (await state()).changes;
await rowOf("Basic steps").locator("> .name").dblclick();
check((await p.locator("#outline .outline-rename").count()) === 0, "readonly: no rename input");
await rowOf("Basic steps").click();
await p.locator("#outline .outline-tree").focus();
await p.keyboard.press("ArrowDown");
const roState = await ol();
check(roState.el !== "Basic steps" && roState.changes === roChanges, "readonly: row click and arrow keys still navigate, no onChange");
check((await p.locator(".tn-bar .tn-seg").count()) === 0, "readonly: no add / remove item buttons");
await p.screenshot({ path: `${out}/21-readonly-nav.png` });

check(problems.length === 0, `no console errors at the end ${problems.slice(0, 3).join(" | ")}`);
await browser.close();
server.close();
if (failures.length > 0) {
  console.error(`${failures.length} check(s) failed`);
  process.exit(1);
}
