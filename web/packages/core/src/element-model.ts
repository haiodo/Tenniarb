// Port of ElementModel.swift. Identity (Swift Hashable by id) is object identity here; ids are kept for parity.
import { TennNode, newBlockExpr } from "./model.ts";

export type ElementKind = "Root" | "Element";
export type ItemKind = "Item" | "Link" | "Annontation";

export function elementCommandName(kind: ElementKind): string {
  return kind === "Root" ? "model" : "element";
}

export function itemCommandName(kind: ItemKind): string {
  switch (kind) {
    case "Item":
      return "item";
    case "Link":
      return "link";
    case "Annontation":
      return "annotation";
  }
}

export class ModelProperties implements Iterable<TennNode> {
  node: TennNode = newBlockExpr();

  constructor(props: TennNode[] = []) {
    this.node.add(...props);
  }

  [Symbol.iterator](): Iterator<TennNode> {
    return (this.node.children ?? [])[Symbol.iterator]();
  }

  append(itm: TennNode): void {
    this.node.add(itm);
  }

  appendContentsOf(nodes: TennNode[]): void {
    for (const c of nodes) {
      this.append(c);
    }
  }

  get(name: string): TennNode | null {
    return this.node.getNamedElement(name);
  }

  get count(): number {
    return this.node.count;
  }

  clone(): ModelProperties {
    const result = new ModelProperties();
    result.node = this.node.clone();
    return result;
  }

  asNode(): TennNode {
    return this.node;
  }
}

// crypto.randomUUID is missing in insecure contexts (file://, plain http); getRandomValues is not.
function newId(): string {
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6] & 0x0f) | 0x40;
  b[8] = (b[8] & 0x3f) | 0x80;
  const h = Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

export class Element {
  readonly id: string = newId();
  kind: ElementKind;
  name: string;
  elements: Element[] = [];
  parent: Element | null = null;
  model: ElementModel | null = null;
  items: DiagramItem[] = [];
  description: string | null = null;
  properties: ModelProperties = new ModelProperties(); // Extra nodes not supported directly by model.

  // Transient data values
  ox = 0;
  oy = 0;

  constructor(name = "") {
    this.name = name;
    this.kind = "Element";
  }

  get asElements(): Element[] | null {
    return this.elements.length === 0 ? null : this.elements;
  }

  /** Doing a full clone of this element with all childrens. */
  clone(cloneItems = true, cloneElement = true): Element {
    const cloneEl = new Element(this.name);

    cloneEl.ox = this.ox;
    cloneEl.oy = this.oy;
    cloneEl.description = this.description;

    cloneEl.properties.appendContentsOf([...this.properties].map((itm) => itm.clone()));

    if (cloneItems) {
      const itemsMap = new Map<DiagramItem, DiagramItem>();
      const linksToProcess: LinkItem[] = [];

      for (const itm of this.items) {
        const copy = itm.clone();
        copy.parent = cloneEl;
        itemsMap.set(itm, copy);
        if (copy instanceof LinkItem) {
          linksToProcess.push(copy);
        }
        cloneEl.items.push(copy);
      }

      // We need to process links with right items
      for (const link of linksToProcess) {
        if (link.source !== null) {
          link.source = itemsMap.get(link.source) ?? null;
        }
        if (link.target !== null) {
          link.target = itemsMap.get(link.target) ?? null;
        }
      }
    }

    if (cloneElement) {
      for (const el of this.elements) {
        const copy = el.clone(true, true);
        copy.parent = cloneEl;
        cloneEl.elements.push(copy);
      }
    }

    return cloneEl;
  }

  get count(): number {
    return this.elements.length;
  }

  get itemCount(): number {
    return this.items.length;
  }

  assignModel(el: Element | DiagramItem): void {
    if (el instanceof DiagramItem) {
      el.parent = this;
      return;
    }
    this.model?.assignModel(el);

    // Assign all childs a proper model
    for (const child of el.elements) {
      this.assignModel(child);
    }
  }

  add(item: Element | DiagramItem, at: number | null = null): void {
    // Swift Array.insert traps on a bad index; fail before touching state.
    if (at !== null && (at < 0 || (item instanceof DiagramItem && at > this.items.length))) {
      throw new RangeError(`add: index ${at} out of range`);
    }
    if (item instanceof DiagramItem) {
      this.assignModel(item);
      if (at !== null) {
        this.items.splice(at, 0, item);
      } else {
        this.items.push(item);
      }
      return;
    }
    item.parent = this;

    this.assignModel(item);
    if (at !== null && this.elements.length > at) {
      this.elements.splice(at, 0, item);
    } else {
      this.elements.push(item);
    }
  }

  /** Add a diagram item to current diagram */
  addGet(item: DiagramItem): DiagramItem {
    this.add(item);
    return item;
  }

  /** Add a child element to current diagram */
  addMakeItem(el: Element): DiagramItem {
    this.add(el);
    const item = new DiagramItem("Item", el.name);
    this.items.push(item);
    this.assignModel(item);

    return item;
  }

  /** Add a link between two items; items missing from the diagram are added after the link. */
  addSourceTarget(source: DiagramItem, target: DiagramItem): void {
    const link = new LinkItem("Link", "", source, target);
    this.items.push(link);
    this.assignModel(link);

    if (!this.items.includes(source)) {
      this.items.push(source);
      this.assignModel(source);
    }
    if (!this.items.includes(target)) {
      this.items.push(target);
      this.assignModel(target);
    }
  }

  /** Returns the removed index, -1 when not found. */
  remove(item: Element | DiagramItem): number {
    const list: (Element | DiagramItem)[] = item instanceof DiagramItem ? this.items : this.elements;
    const index = list.indexOf(item);
    if (index !== -1) {
      list.splice(index, 1);
    }
    return index;
  }

  findLinks(item: DiagramItem): DiagramItem[] {
    const result: DiagramItem[] = [];
    for (const itm of this.items) {
      // Need to check if item is Link and source or target is our client
      if (itm.kind === "Link" && itm instanceof LinkItem) {
        if (itm.source?.id === item.id || itm.target?.id === item.id) {
          result.push(itm);
        }
      }
    }
    return result;
  }

  getRelatedItems(item: DiagramItem, source = true, target = true): DiagramItem[] {
    return this.items.filter((it) => {
      // Need to check if item is Link and source or target is our client
      if (it.kind === "Link" && it instanceof LinkItem) {
        if ((source && it.source?.id === item.id) || (target && it.target?.id === item.id)) {
          return true;
        }
      }
      return it.id === item.id;
    });
  }
}

/** A root of element map */
export class ElementModel extends Element {
  modelName = "";

  constructor() {
    super("Root");
    this.kind = "Root";
  }

  override assignModel(el: Element | DiagramItem): void {
    if (el instanceof DiagramItem) {
      super.assignModel(el);
      return;
    }
    el.model = this;

    // Assign all childs a proper model
    for (const child of el.elements) {
      this.assignModel(child);
    }
  }
}

export class DiagramItem {
  readonly id: string = newId();
  kind: ItemKind;
  name: string;
  parent: Element | null = null;
  description: string | null = null;
  properties: ModelProperties = new ModelProperties(); // Extra nodes not supported directly by model.
  x = 0;
  y = 0;

  constructor(kind: ItemKind, name: string) {
    this.kind = kind;
    this.name = name;
  }

  clone(): DiagramItem {
    const cloneItm = new DiagramItem(this.kind, this.name);

    cloneItm.x = this.x;
    cloneItm.y = this.y;
    cloneItm.description = this.description;

    cloneItm.properties.appendContentsOf([...this.properties].map((nde) => nde.clone()));

    return cloneItm;
  }
}

export class LinkItem extends DiagramItem {
  source: DiagramItem | null; // A direct link to source
  target: DiagramItem | null; // A direct link to target in case of Link

  constructor(kind: ItemKind, name: string, source: DiagramItem | null, target: DiagramItem | null) {
    super(kind, name);
    this.source = source;
    this.target = target;
  }

  override clone(): LinkItem {
    const cloneItm = new LinkItem(this.kind, this.name, this.source, this.target);

    cloneItm.x = this.x;
    cloneItm.y = this.y;
    cloneItm.description = this.description;

    cloneItm.properties.appendContentsOf([...this.properties].map((nde) => nde.clone()));

    return cloneItm;
  }
}

/** Default element factory. */
export class ElementModelFactory {
  elementModel: ElementModel;
  constructor() {
    this.elementModel = new ElementModel();

    const pl = new Element("Unnamed diagram");
    this.elementModel.add(pl);
  }
}
