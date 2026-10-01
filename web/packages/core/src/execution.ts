// Port of ExecutionModel.swift. Main thread only: syncQueue, the *NoSync variants and ExecutionContextEval are dropped.
// Expressions run on the platform engine: eval in `with` over a Proxy of a per-ElementContext global map, inside a
// generator frame that outlives each call, so `function`/`var` from one expression stay visible to the next.
// Ceiling: no sandbox (host builtins visible, loops hang). Untrusted: `evaluate: false`.
import type { TennNode, TennToken } from "./model.ts";
import { DiagramItem, LinkItem, itemCommandName } from "./element-model.ts";
import type { Element } from "./element-model.ts";
import type { IExecutionContext, ModelEvent } from "./element-operations.ts";
import { traverseBlock } from "./persistence.ts";

export interface Size {
  width: number;
  height: number;
}

/** Value the scene reads per token: the JS result, or the thrown value when the expression failed. */
export type EvaluatedValues = ReadonlyMap<TennToken, unknown>;

export interface ExecutionContextEvaluator {
  getEvaluated(target: Element | DiagramItem): EvaluatedValues;
}

export interface ExecutionOptions {
  /** false: nothing runs and getEvaluated is always empty, so callers see raw token text. */
  evaluate?: boolean;
  /** Stands in for the drawable's bounds; without it `width`, `height`, `defaults` are undefined. */
  itemSize?: (item: DiagramItem) => Size | null;
  /** Raw text frame size (CTFramesetter in Swift); utils.textSize adds the padding on top. */
  measureText?: (text: string, fontSize: number) => Size;
}

// Swift value kinds that checkChanges and sum tell apart; JS numbers cannot carry Int vs Double.
class JSValue {
  readonly value: unknown;
  constructor(value: unknown) {
    this.value = value;
  }
}
class SwiftDouble {
  readonly value: number;
  constructor(value: number) {
    this.value = value;
  }
}
class PositionItem {
  readonly x: number;
  readonly y: number;
  constructor(x: number, y: number) {
    this.x = x;
    this.y = y;
  }
}
type SwiftDict = Map<string, SwiftValue>;
type SwiftValue = string | number | SwiftDouble | JSValue | PositionItem | SwiftDict | (SwiftValue | null)[];

// Like setObject: native containers are copied, JSValues pass through.
function toJS(v: SwiftValue | null): unknown {
  if (v instanceof JSValue || v instanceof SwiftDouble) {
    return v.value;
  }
  if (v instanceof PositionItem) {
    return Object.freeze({ x: v.x, y: v.y });
  }
  if (v instanceof Map) {
    return Object.fromEntries(Array.from(v, ([k, x]) => [k, toJS(x)]));
  }
  if (Array.isArray(v)) {
    return v.map((x) => toJS(x));
  }
  return v;
}

const alphanumeric = /[\p{L}\p{M}\p{N}]/u;
function convertNameJS(name: string): string | null {
  let result = "";
  for (const c of name) {
    result += alphanumeric.test(c) ? c : "_";
  }
  return result.length > 0 ? result : null;
}

const identifier = /^[\p{ID_Start}$_][\p{ID_Continue}$‌‍]*$/u;
const reserved = new Set(
  (
    "break case catch class const continue debugger default delete do else enum export extends false finally for function " +
    "if import in instanceof new null return super switch this throw true try typeof var void while with"
  ).split(" "),
);

const GeneratorFunction = Object.getPrototypeOf(function* () {}).constructor as new (...args: string[]) => (scope: object) => Generator<
  unknown,
  void,
  string
>;

type Completion = { v: unknown } | { e: unknown };

// Stand-in for the JSContext of one ElementContext; `scope` is its global object.
class JSContext {
  readonly scope = new Map<string, unknown>();
  private readonly frame: Generator<unknown, void, string>;

  constructor() {
    const scope = this.scope;
    // Claims every name except those declared in the frame by eval (functions, hoisted vars), so implicit globals land in
    // the map and never on globalThis. Cost: `typeof unknownName` throws ReferenceError instead of "undefined".
    let declared: (name: string) => boolean = () => false;
    const globals = new Proxy(
      {},
      {
        has: (_, k) => typeof k === "string" && (scope.has(k) || k in globalThis || !declared(k)),
        get: (_, k) => {
          if (typeof k !== "string") {
            return undefined;
          }
          if (scope.has(k)) {
            return scope.get(k);
          }
          if (k in globalThis) {
            return (globalThis as Record<string, unknown>)[k];
          }
          throw new ReferenceError(`Can't find variable: ${k}`);
        },
        set: (_, k, v) => {
          scope.set(k as string, v);
          return true;
        },
        deleteProperty: (_, k) => {
          scope.delete(k as string);
          return true;
        },
      },
    );
    // Function constructor bodies are sloppy, so `with` is allowed. A throw would end the generator, hence the try.
    this.frame = new GeneratorFunction(
      "__tenn",
      "function __probe(n) { return eval('typeof ' + n) !== 'undefined'; } yield __probe; with (__tenn) { var __c = yield; for (;;) { try { __c = { v: eval(__c) }; } catch (e) { __c = { e }; } __c = yield __c; } }",
    )(globals);
    declared = this.frame.next().value as unknown as (name: string) => boolean;
    this.frame.next();
  }

  evaluateScript(code: string): Completion {
    return this.frame.next(code).value as Completion;
  }

  // Swift runs `delete name` as a script, which is a SyntaxError (nothing deleted) for non-identifiers and keywords.
  deleteName(name: string): void {
    if (identifier.test(name) && !reserved.has(name)) {
      this.scope.delete(name);
    }
  }
}

function runScript(ctx: JSContext, code: string, token: TennToken | null, evaluated: Map<TennToken, unknown>): JSValue {
  const r = ctx.evaluateScript(code);
  const failed = "e" in r;
  if (token !== null) {
    evaluated.set(token, failed ? r.e : r.v);
  }
  return new JSValue(failed ? undefined : r.v);
}

function processBlock(node: TennNode, ctx: JSContext, levelObject: SwiftDict, evaluated: Map<TennToken, unknown>): boolean {
  const has = { value: false };
  traverseBlock(node, (cmdNameRaw, blChild) => {
    const cmdName = convertNameJS(cmdNameRaw);
    if (cmdName === null) {
      return;
    }
    if (blChild.count > 1) {
      if (blChild.count === 2) {
        const ci = blChild.getChild(1);
        has.value = has.value || ci?.kind === "ExpressionBlock" || ci?.kind === "Expression";
        // We have simple field assignement, not array
        const value = calculateValue(ci, ctx, levelObject, has, evaluated);
        if (value !== null) {
          levelObject.set(cmdName, value);
          // An empty block is stored but not published to the context.
          if (!(value instanceof Map) || value.size > 0) {
            ctx.scope.set(cmdName, toJS(value));
          }
        }
      } else {
        // We have array assignement
        const values: (SwiftValue | null)[] = [];
        for (let i = 1; i < blChild.count; i++) {
          const ci = blChild.getChild(i);
          has.value = has.value || ci?.kind === "ExpressionBlock" || ci?.kind === "Expression";
          values.push(calculateValue(ci, ctx, levelObject, has, evaluated));
        }
        levelObject.set(cmdName, values);
        ctx.scope.set(cmdName, toJS(values));
      }
    } else {
      // Set empty value, to mark field defined
      levelObject.set(cmdName, "");
      ctx.scope.set(cmdName, "");
    }
  });
  return has.value;
}

function calculateValue(
  nde: TennNode | null,
  ctx: JSContext,
  currentScope: SwiftDict,
  has: { value: boolean },
  evaluated: Map<TennToken, unknown>,
): SwiftValue | null {
  if (nde === null) {
    return null;
  }
  const identText = nde.getIdentText();
  switch (nde.kind) {
    case "FloatLit": {
      if (identText === null) {
        return null;
      }
      const d = Number(identText);
      return new SwiftDouble(Number.isNaN(d) ? 0 : d);
    }
    case "IntLit":
      if (identText === null) {
        return null;
      }
      // Swift Int(String) is 64-bit and strict; anything else falls back to 0.0.
      return /^[+-]?\d+$/.test(identText) && BigInt.asIntN(64, BigInt(identText)) === BigInt(identText) ? Number(identText) + 0 : new SwiftDouble(0);
    case "StringLit":
    case "CharLit":
    case "Ident":
    case "MarkdownLit":
      if (identText === null) {
        return null;
      }
      if (identText.includes("${")) {
        has.value = true;
        // We need to perform substituion
        return runScript(ctx, "`" + identText + "`", nde.token, evaluated);
      }
      return identText;
    case "BlockExpr": {
      // A subcontext need to be constructed
      const blockScope: SwiftDict = new Map();
      const he = processBlock(nde, ctx, blockScope, evaluated);
      // We need to cleanup current context from inner scope values
      for (const k of blockScope.keys()) {
        const csv = currentScope.get(k);
        if (csv !== undefined) {
          ctx.scope.set(k, toJS(csv));
        }
      }
      has.value = has.value || he;
      return blockScope;
    }
    case "ExpressionBlock":
    case "Expression":
      has.value = true;
      return identText === null ? null : runScript(ctx, identText, nde.token, evaluated);
    default:
      return null;
  }
}

class UtilsContext {
  readonly now = (): number => Date.now() / 1000;
  readonly textWidth: (text: string, fontSize?: unknown) => number;
  readonly textSize: (text: string, fontSize?: unknown) => number[];

  constructor(measureText: (text: string, fontSize: number) => Size) {
    this.textSize = (text, fontSize) => {
      const size = measureText(String(text), typeof fontSize === "number" ? fontSize : 18);
      return [size.width + 6, size.height + 4];
    };
    this.textWidth = (text, fontSize) => this.textSize(text, fontSize)[0]!;
  }
}

// Placeholder until a real text measurer (stage 5) is passed in: average glyph width, one line.
function estimateText(text: string, fontSize: number): Size {
  return { width: text.length * fontSize * 0.55, height: fontSize * 1.2 };
}

// Swift round(): half away from zero.
const round = (x: number): number => Math.sign(x) * Math.round(Math.abs(x));

class ItemContext {
  item: DiagramItem;
  itemObject: SwiftDict = new Map(); // To be used from references
  hasExpressions = false;
  parentCtx: ElementContext;

  evaluated = new Map<TennToken, unknown>();

  // Filled by sum/edges/inputs/outputs/items but never read, as in Swift.
  dependencies = new Set<ItemContext>();

  constructor(parentCtx: ElementContext, item: DiagramItem) {
    this.item = item;
    this.parentCtx = parentCtx;
  }

  updateContext(): boolean {
    const newItems: SwiftDict = new Map();
    const newEvaluated = new Map<TennToken, unknown>();

    this.hasExpressions = this.updateGetContext(null, newItems, newEvaluated, this.parentCtx.context.itemSize?.(this.item) ?? null);

    // Check if we had value changes
    const result = this.checkChanges(this.itemObject, newItems);

    this.itemObject = newItems;
    this.evaluated = newEvaluated;

    return result;
  }

  // Only String, Int and JSValue entries are compared; Doubles, arrays, blocks and pos are not (as in Swift).
  checkChanges(oldItems: SwiftDict, newItems: SwiftDict): boolean {
    if (oldItems.size !== newItems.size) {
      return true;
    }
    for (const [k, v] of oldItems) {
      const nk = newItems.get(k);
      if (nk === undefined) {
        // Not pressent, we had changed.
        return true;
      }
      if (nk instanceof JSValue && v instanceof JSValue) {
        if (jsToString(nk.value) !== jsToString(v.value)) {
          return true;
        }
      } else if (typeof nk === typeof v && (typeof nk === "string" || typeof nk === "number") && nk !== v) {
        return true;
      }
    }
    return false;
  }

  getRelativeItems(source: boolean, target: boolean): unknown[] {
    const result: unknown[] = [];
    for (const itm of this.parentCtx.element.getRelatedItems(this.item, source, target)) {
      if (itm.id === this.item.id || itm.kind !== "Link") {
        continue;
      }
      if (!(itm instanceof LinkItem)) {
        continue;
      }
      const opposite = itm.source === this.item ? itm.target : itm.source;
      if (opposite !== null) {
        const ictx = this.parentCtx.itemsMap.get(opposite);
        if (ictx !== undefined) {
          result.push(toJS(ictx.itemObject));
        }
      }
    }
    return result;
  }

  private registerSum(): void {
    const sum = (tagName: unknown, fieldName?: unknown): number => {
      const tag = String(tagName);
      // Swift only reads a String second argument; anything else means the tag itself.
      const field = typeof fieldName === "string" && fieldName.length > 0 ? fieldName : tag;
      let result = 0;
      for (const e of this.parentCtx.itemsMap.values()) {
        if (!e.itemObject.has(tag)) {
          continue;
        }
        this.dependencies.add(e);
        const vv = e.itemObject.get(field);
        if (typeof vv === "number") {
          result += vv;
        } else if (vv instanceof SwiftDouble) {
          result += vv.value;
        } else if (vv instanceof JSValue) {
          result += Number(vv.value);
        }
      }
      return result;
    };

    this.parentCtx.jsContext.scope.set("sum", sum);
  }

  private registerByTag(): void {
    const byTag = (tagName: unknown): unknown[] => {
      const tag = String(tagName);
      // Make it in right order every time.
      const result: unknown[] = [];
      for (const e of this.parentCtx.element.items) {
        const ic = this.parentCtx.itemsMap.get(e);
        if (ic !== undefined && ic.itemObject.has(tag)) {
          result.push(toJS(ic.itemObject));
        }
      }
      return result;
    };

    this.parentCtx.jsContext.scope.set("byTag", byTag);
  }

  private registerInputsOutputs(): void {
    // Edge operations
    const related = (source: boolean, target: boolean) => (): unknown[] => {
      for (const itm of this.parentCtx.element.getRelatedItems(this.item, source, target)) {
        const ctx = this.parentCtx.itemsMap.get(itm);
        if (ctx !== undefined) {
          this.dependencies.add(ctx);
        }
      }
      return this.getRelativeItems(source, target);
    };

    const scope = this.parentCtx.jsContext.scope;
    scope.set("edges", related(true, true));
    scope.set("inputs", related(false, true));
    scope.set("outputs", related(true, false));
  }

  private registerItems(): void {
    const valueForKey = (_target: unknown, key: unknown): unknown => {
      const itm = typeof key === "string" ? this.parentCtx.namedItems.get(key) : undefined;
      if (itm !== undefined) {
        this.dependencies.add(itm);
        return toJS(itm.itemObject);
      }
      return undefined;
    };

    const scope = this.parentCtx.jsContext.scope;
    scope.set("__valueForKey", valueForKey);
    scope.set("items", new Proxy({}, { get: valueForKey }));
  }

  private cleanContext(): void {
    // We need to set old values to be empty
    for (const k of this.itemObject.keys()) {
      this.parentCtx.jsContext.deleteName(k);
    }
  }

  updateGetContext(node: TennNode | null, newItems: SwiftDict, newEvaluated: Map<TennToken, unknown>, drawable: Size | null): boolean {
    this.cleanContext();

    const scope = this.parentCtx.jsContext.scope;
    this.registerItems();

    const parentCtx = this.parentCtx;
    const itemsOfKind = (kind: DiagramItem["kind"]): unknown[] => {
      const result: unknown[] = [];
      for (const e of parentCtx.element.items) {
        const ic = parentCtx.itemsMap.get(e);
        if (ic !== undefined && e.kind === kind) {
          result.push(toJS(ic.itemObject));
        }
      }
      return result;
    };
    scope.set("parent", {
      get items() {
        return itemsOfKind("Item");
      },
      get links() {
        return itemsOfKind("Link");
      },
    });
    scope.set("utils", parentCtx.utils);

    // Update position
    const posObj = new PositionItem(this.item.x, this.item.y);
    scope.set("pos", toJS(posObj));

    if (drawable !== null) {
      const width = round(drawable.width);
      const height = round(drawable.height);
      scope.set("defaults", Object.freeze({ width, height }));
      scope.set("width", width);
      scope.set("height", height);

      newItems.set("width", new SwiftDouble(width));
      newItems.set("height", new SwiftDouble(height));
    }

    // Update name
    scope.set("name", this.item.name);
    scope.set("kind", itemCommandName(this.item.kind));
    scope.set("id", this.item.id.toUpperCase()); // Swift UUID.uuidString
    newItems.set("name", this.item.name);
    newItems.set("pos", posObj);

    this.registerInputsOutputs();

    this.registerByTag();
    this.registerSum();

    const result = processBlock(node ?? this.item.properties.node, this.parentCtx.jsContext, newItems, newEvaluated);

    // Cleanup most of context
    this.cleanContext();
    return result;
  }
}

// JSValue.toString()
function jsToString(v: unknown): string {
  try {
    return String(v);
  } catch {
    return "";
  }
}

export class ElementContext {
  element: Element;
  elementObject: SwiftDict = new Map();
  context: ExecutionContext;
  jsContext = new JSContext();
  hasExpressions = false;
  evaluated = new Map<TennToken, unknown>();
  itemsMap = new Map<DiagramItem, ItemContext>();

  namedItems = new Map<string, ItemContext>();

  utils: UtilsContext;

  private reCalculate(withExprs: ItemContext[]): DiagramItem[] {
    let iterations = 100;
    const changes = new Set<DiagramItem>();
    const toCheck = [...withExprs];
    //TODO: Add more smart cycle detection logic
    while (iterations > 0 && toCheck.length > 0) {
      let changed = 0;
      for (const ic of toCheck) {
        if (ic.hasExpressions && ic.updateContext()) {
          changes.add(ic.item);
          changed += 1;
        }
      }
      if (changed === 0) {
        break;
      }
      iterations -= 1;
    }
    return [...changes];
  }

  constructor(context: ExecutionContext, element: Element) {
    this.element = element;
    this.context = context;
    this.utils = new UtilsContext(context.measureText);

    this.updateContext();

    const withExprs: ItemContext[] = [];
    for (const itm of element.items) {
      const ic = new ItemContext(this, itm);
      ic.updateContext();
      this.itemsMap.set(itm, ic);
      if (ic.hasExpressions) {
        withExprs.push(ic);
      }
      this.namedItems.set(convertNameJS(itm.name) ?? itm.name, ic);
    }
    if (this.hasExpressions) {
      this.updateContext();
    }

    this.reCalculate(withExprs);
  }

  updateGetContext(node: TennNode | null, newItems: SwiftDict, newEvaluated: Map<TennToken, unknown>): boolean {
    // We need to set old values to be empty
    for (const k of this.elementObject.keys()) {
      this.jsContext.deleteName(k);
    }

    this.jsContext.scope.set("utils", this.utils);

    return processBlock(node ?? this.element.properties.node, this.jsContext, newItems, newEvaluated);
  }

  updateContext(node: TennNode | null = null): void {
    const newItems: SwiftDict = new Map();
    const newEvaluated = new Map<TennToken, unknown>();

    this.hasExpressions = this.updateGetContext(node, newItems, newEvaluated);

    this.elementObject = newItems;
    this.evaluated = newEvaluated;
  }

  processEvent(event: ModelEvent): void {
    let needUpdateNamed = false;
    for (const [k, v] of event.items) {
      switch (v) {
        case "Append": {
          // New item we need to add it to calculation
          const ikc = new ItemContext(this, k);
          ikc.updateContext();
          this.itemsMap.set(k, ikc);
          needUpdateNamed = true;
          break;
        }
        case "Remove":
          if (this.itemsMap.delete(k)) {
            this.namedItems.delete(convertNameJS(k.name) ?? k.name);
          }
          needUpdateNamed = true;
          break;
        case "Update":
          this.itemsMap.get(k)?.updateContext();
          // Probable we need to update list of named items.
          needUpdateNamed = true;
          break;
      }
    }

    if (needUpdateNamed) {
      this.namedItems.clear();
      for (const itm of this.element.items) {
        const itmVal = this.itemsMap.get(itm);
        const name = convertNameJS(itm.name) ?? itm.name;
        if (itmVal === undefined) {
          this.namedItems.delete(name);
        } else {
          this.namedItems.set(name, itmVal);
        }
      }
    }

    const withExprs = [...this.itemsMap.values()].filter((e) => e.hasExpressions);

    for (const c of this.reCalculate(withExprs)) {
      if (!event.items.has(c)) {
        event.items.set(c, "Update");
      }
    }
  }
}

export class ExecutionContext implements ExecutionContextEvaluator, IExecutionContext {
  elements = new Map<Element, ElementContext>();
  rootCtx: ElementContext | null = null;

  scaleFactor = 1;
  readonly evaluate: boolean;
  readonly itemSize: ((item: DiagramItem) => Size | null) | null;
  readonly measureText: (text: string, fontSize: number) => Size;

  constructor(options: ExecutionOptions = {}) {
    this.evaluate = options.evaluate ?? true;
    this.itemSize = options.itemSize ?? null;
    this.measureText = options.measureText ?? estimateText;
  }

  setScaleFactor(factor: number): void {
    this.scaleFactor = factor;
  }

  setElement(element: Element): void {
    if (!this.evaluate) {
      return;
    }
    // rootCtx is assigned after construction, so getEvaluated is empty while the context builds (as in Swift).
    this.rootCtx = new ElementContext(this, element);
    this.elements.set(element, this.rootCtx);
  }

  // Synchronous here; Swift hops to a utility queue and calls notifier from it.
  updateAll(notifier: () => void): void {
    if (!this.evaluate) {
      return; // like Swift without a root
    }
    const root = this.rootCtx;
    if (root !== null) {
      root.updateContext();
      // We need to recalculate all stuff
      for (const ci of root.itemsMap.values()) {
        if (ci.hasExpressions) {
          ci.updateContext();
        }
      }
      notifier();
    }
  }

  notifyChanges(event: ModelEvent): void {
    if (this.rootCtx !== null && event.element === this.rootCtx.element) {
      this.rootCtx.processEvent(event);
    }
  }

  /** With `node` it re-evaluates that node in the item/element scope (editor preview) and returns only its values. */
  getEvaluated(target: Element | DiagramItem, node?: TennNode, drawable: Size | null = null): EvaluatedValues {
    const root = this.rootCtx;
    if (root === null) {
      return new Map();
    }
    if (target instanceof DiagramItem) {
      const ic = root.itemsMap.get(target);
      if (ic === undefined) {
        return new Map();
      }
      if (node === undefined) {
        return ic.evaluated;
      }
      const value = new Map<TennToken, unknown>();
      ic.updateGetContext(node, new Map(), value, drawable);
      return value;
    }
    if (root.element !== target) {
      return new Map();
    }
    if (node === undefined) {
      return root.evaluated;
    }
    const value = new Map<TennToken, unknown>();
    root.updateGetContext(node, new Map(), value);
    return value;
  }
}
