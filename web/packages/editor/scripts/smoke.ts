// Playwright smoke: node scripts/smoke.ts [outDir]. Not part of CI; Playwright comes from web/bench/node_modules.
// Needs dist/ (npm run build -w @tenniarb/editor).
import { createServer } from "node:http";
import { mkdirSync, readFileSync } from "node:fs";
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
await p.mouse.click(1050, 760);
check((await state()).sel === 0, "click on empty space clears selection");
const before = await p.$eval("canvas", (c: HTMLCanvasElement) => c.toDataURL());
await p.mouse.move(1050, 760);
await p.mouse.down();
await p.mouse.move(1000, 700, { steps: 4 });
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
await p.mouse.move(1050, 760);
await p.mouse.down();
await p.mouse.move(1000, 700, { steps: 4 });
await p.mouse.up();
const panned = await p.$eval("canvas", (c: HTMLCanvasElement) => c.toDataURL());
await p.mouse.dblclick(1060, 770);
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

// outline: switch, rename, add, delete, drag, search
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

await rowOf("Basic steps").click();
await p.fill('#outline input[type="search"]', "Main Window");
await p.waitForTimeout(200);
const sq = await p.evaluate(() => ({ hits: document.querySelectorAll("#outline .outline-results .row").length, shown: !document.querySelector(".outline-results")!.classList.contains("hidden"), treeHidden: document.querySelector(".outline-tree")!.classList.contains("hidden"), first: document.querySelector("#outline .outline-results .row")?.textContent }));
const o7 = await ol();
check(sq.hits > 0 && sq.shown && sq.treeHidden, `search lists hits instead of the tree (${sq.hits}: "${sq.first}")`);
check(o7.el !== "Basic steps" && o7.sels[0] === "Main Window", `the first hit switched to "${o7.el}" and selected "${o7.sels[0]}"`);
await p.screenshot({ path: `${out}/20-search.png` });
await p.keyboard.press("Escape");
const sc = await p.evaluate(() => ({ hidden: document.querySelector(".outline-results")!.classList.contains("hidden"), value: (document.querySelector('#outline input[type="search"]') as HTMLInputElement).value, focus: document.activeElement === document.querySelector("#editor > div") }));
check(sc.hidden && sc.value === "" && sc.focus && (await ol()).sels[0] === "Main Window", "Esc closes the search, focus returns to the canvas, the selection stays");

// properties panel: long lines scroll (no wrap)
const sc0 = await p.evaluate(() => { const e = document.querySelector("#props .cm-scroller") as HTMLElement; const before = e.scrollLeft; e.scrollLeft = 100; return { sw: e.scrollWidth, cw: e.clientWidth, ox: getComputedStyle(e).overflowX, moved: e.scrollLeft - before }; });
check(sc0.sw > sc0.cw && sc0.ox === "auto" && sc0.moved > 0, `panel scrolls horizontally (${sc0.sw} > ${sc0.cw}, moved ${sc0.moved})`);
await p.screenshot({ path: `${out}/30-panel-scroll.png` });

// styles toolbar
const st = () => p.evaluate(() => { const s = (window as any).editor.session; const it = s.selection[0]; return { model: s.text() as string, changes: (window as any).changes as number, styles: s.styleNames() as string[], has: it ? s.propsText(it) as string : "" }; });
const tn = (name: string) => p.locator(`.tn-toolbar [data-tn="${name}"]`);
const t0 = await st();
check((await tn("shadows").count()) === 0 && (await tn("color").count()) === 1, "toolbar: quick styles for the selected item");
await tn("color").selectOption("red");
await p.waitForTimeout(100);
const t1 = await st();
check(t1.has.includes("color red") && t1.changes === t0.changes + 1 && t1.model.includes("color red"), "quick style: color red in the model, onChange +1");
await tn("display").selectOption("circle");
await p.waitForTimeout(100);
check((await st()).has.includes("display circle"), "quick style: display circle");
await p.screenshot({ path: `${out}/31-styles.png` });
await p.keyboard.press("ControlOrMeta+z");
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(100);
const t2 = await st();
check(!t2.has.includes("color red") && !t2.has.includes("display circle") && t2.changes === t0.changes + 4, "undo x2 restores the item");
await tn("style").selectOption("+ New style");
await p.waitForTimeout(100);
const t3 = await st();
check(t3.styles.length === t0.styles.length + 1 && t3.changes === t2.changes + 1, `new style defined (${t3.styles.join(",")})`);
const newName = t3.styles[t3.styles.length - 1]!;
await tn("style").selectOption(newName);
await p.waitForTimeout(100);
const t4 = await st();
check(t4.has.includes(`use-style ${newName}`) && t4.changes === t3.changes + 1, "style applied to the selection, one onChange");
await p.keyboard.press("ControlOrMeta+z");
await p.waitForTimeout(100);
check(!(await st()).has.includes("use-style"), "undo removes use-style");
await p.evaluate(() => { const s = (window as any).editor.session; s.selection = []; s.opts.onRedraw(); });
await p.waitForTimeout(100);
check((await tn("shadows").count()) === 1 && (await tn("color").count()) === 0, "no selection: only style and shadows");

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
check((await p.locator(".tn-toolbar").count()) === 0, "readonly: no toolbar");
check(sr.sel === 0 && sr.x === s0.x && sr.y === s0.y && sr.changes === ro0, "readonly: no selection, no move, no onChange");
await p.screenshot({ path: `${out}/5-readonly.png` });
check((await p.locator("#outline .outline-header button").count()) === 0, "readonly: no outline buttons");
const roChanges = (await state()).changes;
await rowOf("Basic steps").locator("> .name").dblclick();
check((await p.locator("#outline .outline-rename").count()) === 0, "readonly: no rename input");
await rowOf("Basic steps").click();
await p.fill('#outline input[type="search"]', "Main Window");
await p.waitForTimeout(200);
const roState = await ol();
check(roState.el !== "Basic steps" && roState.sels[0] === "Main Window" && roState.changes === roChanges, "readonly: row click and search still navigate, no onChange");
await p.screenshot({ path: `${out}/21-readonly-search.png` });

check(problems.length === 0, `no console errors at the end ${problems.slice(0, 3).join(" | ")}`);
await browser.close();
server.close();
if (failures.length > 0) {
  console.error(`${failures.length} check(s) failed`);
  process.exit(1);
}
