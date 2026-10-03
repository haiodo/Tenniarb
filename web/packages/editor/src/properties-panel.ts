// Properties panel on CodeMirror 6; port of TextPropertiesDelegate (text of the selected item, delayed apply, grey expression values).
import { EditorState, StateEffect, StateField } from "@codemirror/state";
import type { Extension } from "@codemirror/state";
import { Decoration, EditorView, WidgetType, keymap } from "@codemirror/view";
import type { DecorationSet } from "@codemirror/view";
import { defaultKeymap, insertNewlineAndIndent } from "@codemirror/commands";
import { indentService } from "@codemirror/language";
import { linter } from "@codemirror/lint";
import { TennParser, toStr } from "@tenniarb/core";
import type { DiagramItem, Element } from "@tenniarb/core";
import { COLORS, highlightRanges, parseDiagnostics } from "./properties.ts";
import type { EditorSession } from "./session.ts";

export interface PropertiesPanel {
  /** Follow the session: selection, model changes. `force` re-evaluates the expressions even when the text is unchanged. */
  sync(force?: boolean): void;
  destroy(): void;
}

const APPLY_DELAY = 300;
const INDENT = "    ";

class ValueWidget extends WidgetType {
  readonly text: string;
  constructor(text: string) {
    super();
    this.text = text;
  }
  override eq(o: ValueWidget): boolean {
    return o.text === this.text;
  }
  override toDOM(): HTMLElement {
    const el = document.createElement("span");
    el.className = "tn-value";
    el.textContent = this.text.replaceAll("\n", "\\n");
    return el;
  }
}

const setValues = StateEffect.define<Map<number, string>>();
const values = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(deco, tr) {
    if (tr.docChanged) deco = Decoration.none; // a value of an edited text would be stale
    for (const e of tr.effects) {
      if (!e.is(setValues)) continue;
      const doc = tr.state.doc;
      const widgets = [...e.value].filter(([line]) => line < doc.lines).map(([line, v]) => Decoration.widget({ widget: new ValueWidget(v), side: 1 }).range(doc.line(line + 1).to));
      return Decoration.set(widgets, true);
    }
    return deco;
  },
  provide: (f) => EditorView.decorations.from(f),
});

// The lexer runs over the whole text, so multi-line strings and expressions stay correct.
const highlight = StateField.define<DecorationSet>({
  create: (state) => tokens(state.doc.toString()),
  update: (deco, tr) => (tr.docChanged ? tokens(tr.newDoc.toString()) : deco),
  provide: (f) => EditorView.decorations.from(f),
});
const tokens = (text: string): DecorationSet => Decoration.set(highlightRanges(text).map((r) => Decoration.mark({ class: `tn-${r.cls}` }).range(r.from, r.to)), true);

// Indent of the text before the cursor (or the last non-blank line above), one level deeper after `{`, one less before `}`.
const indent = indentService.of((ctx, pos) => {
  const { doc } = ctx.state;
  const cur = doc.lineAt(pos);
  let text = cur.text.slice(0, pos - cur.from);
  for (let n = cur.number - 1; text.trim() === "" && n > 0; n--) text = doc.line(n).text;
  const base = text.length - text.trimStart().length;
  const deeper = text.trimEnd().endsWith("{") ? INDENT.length : 0;
  const closing = ctx.textAfterPos(pos).trimStart().startsWith("}") ? INDENT.length : 0;
  return Math.max(base + deeper - closing, 0);
});

function theme(dark: boolean): Extension {
  const c = COLORS[dark ? "dark" : "light"];
  return EditorView.theme(
    {
      "&": { height: "100%", fontSize: "13px", backgroundColor: "transparent" },
      "&.cm-focused": { outline: "none" },
      ".cm-scroller": { fontFamily: "ui-monospace, Menlo, monospace", overflow: "auto" },
      // No wrap, as in Swift; styled scrollbars stay visible where overlay ones hide, so long lines are visibly scrollable.
      ".cm-scroller::-webkit-scrollbar": { width: "10px", height: "10px" },
      ".cm-scroller::-webkit-scrollbar-thumb": { background: dark ? "#555" : "#c1c1c1", borderRadius: "5px" },
      ".tn-symbol": { color: c.symbol },
      ".tn-string": { color: c.string },
      ".tn-number": { color: c.number },
      ".tn-expression": { color: c.expression },
      ".tn-value": { color: "gray", marginLeft: "2em", userSelect: "none", pointerEvents: "none" },
    },
    { dark },
  );
}

export function mountProperties(host: HTMLElement, session: EditorSession, opts: { darkMode?: boolean; readonly?: boolean } = {}): PropertiesPanel {
  let target: DiagramItem | Element | null = null;
  let dirty = false; // typed, not applied yet
  let setting = false; // programmatic document replacement
  let timer: ReturnType<typeof setTimeout> | undefined;

  const doc = (): string => view.state.doc.toString();
  const annotate = (): void => {
    if (target !== null) view.dispatch({ effects: setValues.of(session.propsValues(target, doc())) });
  };
  function apply(t: DiagramItem | Element): void {
    clearTimeout(timer);
    // `dirty` stays set while the store notifies, so sync() does not replace the text under the user.
    dirty = !session.applyProps(t, doc());
    if (!dirty && t === target) annotate();
  }
  function replace(text: string): void {
    setting = true;
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text }, selection: { anchor: 0 } });
    setting = false;
  }

  const view: EditorView = new EditorView({
    parent: host,
    state: EditorState.create({
      doc: "",
      extensions: [
        theme(opts.darkMode ?? false),
        highlight,
        values,
        indent,
        linter((v) => parseDiagnostics(v.state.doc.toString()).map((d) => ({ ...d, severity: "error" as const })), { delay: APPLY_DELAY }),
        keymap.of([
          { key: "Tab", run: (v) => (v.dispatch({ ...v.state.replaceSelection(INDENT), userEvent: "input" }), true) },
          { key: "Enter", run: insertNewlineAndIndent },
          ...defaultKeymap,
        ]),
        EditorState.readOnly.of(opts.readonly === true),
        EditorView.editable.of(opts.readonly !== true),
        EditorView.updateListener.of((u) => {
          if (!u.docChanged || setting || target === null) return;
          dirty = true;
          clearTimeout(timer);
          const t = target;
          timer = setTimeout(() => apply(t), APPLY_DELAY);
        }),
        EditorView.domEventHandlers({
          blur: () => {
            if (dirty && target !== null) apply(target);
          },
        }),
      ],
    }),
  });

  return {
    sync(force = false) {
      const next = session.propsTarget();
      if (next !== target) {
        const old = target;
        target = next;
        if (dirty && old !== null) apply(old);
        dirty = false;
      }
      if (dirty) return;
      const text = session.propsText(target);
      // Compare normalized: the panel text may be formatted differently from what the model prints.
      if (toStr(new TennParser().parse(doc()), 0, false) !== text) {
        replace(text);
        annotate();
      } else if (force) annotate();
    },
    destroy() {
      clearTimeout(timer);
      view.destroy();
    },
  };
}
