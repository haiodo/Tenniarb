// Port of ElementPersistence.swift. Swift extension methods on Element are free functions here.
import { TennNode, newBlockExpr, newCommand, newFloatNode, newIntNode, newStrNode } from "./model.ts";
import type { TennNodeKind } from "./model.ts";
import { toStr } from "./printer.ts";
import { DiagramItem, Element, ElementModel, LinkItem } from "./element-model.ts";

export type PersistenceItemKind =
  | "Item"
  | "Link"
  | "Element"
  | "Model"
  | "Annontation"
  | "Description"
  | "Label"
  | "SourceIndex"
  | "TargetIndex"
  | "Name"
  | "Position";

export function persistenceItemCommandName(kind: PersistenceItemKind): string {
  switch (kind) {
    case "Item":
      return "item";
    case "Link":
      return "link";
    case "Element":
      return "element";
    case "Model":
      return "model";
    case "Annontation":
      return "annotation";
    case "Description":
      return "description";
    case "SourceIndex":
      return "source-index";
    case "TargetIndex":
      return "target-index";
    case "Position":
      return "pos";
    case "Name":
      return "name";
    case "Label":
      return "label";
  }
}

export type PersistenceStyleKind =
  | "Color"
  | "TextColor"
  | "FontSize"
  | "Display"
  | "Layout"
  | "LineStyle"
  | "Width"
  | "Height"
  | "BorderColor"
  | "ZoomLevel"
  | "Styles"
  | "Grid"
  | "Title"
  | "Label"
  | "Shadow"
  | "LineWidth"
  | "Marker"
  | "Layer"
  | "Inherit"
  | "UseStyle"
  | "FieldName"
  | "CornerRadius"
  | "LineSpacing";

export function persistenceStyleName(kind: PersistenceStyleKind): string {
  switch (kind) {
    case "Color":
      return "color";
    case "FontSize":
      return "font-size";
    case "Display":
      return "display";
    case "Layout":
      return "layout";
    case "LineStyle":
      return "line-style";
    case "Width":
      return "width";
    case "Height":
      return "height";
    case "BorderColor":
      return "border-color";
    case "ZoomLevel":
      return "zoom";
    case "TextColor":
      return "text-color";
    case "Styles":
      return "styles";
    case "Grid":
      return "grid";
    case "Title":
      return "title";
    case "Label":
      return "label";
    case "Shadow":
      return "shadow";
    case "LineWidth":
      return "line-width";
    case "Marker":
      return "marker";
    case "Layer":
      return "layer";
    case "Inherit":
      return "inherit";
    case "UseStyle":
      return "use-style";
    case "FieldName":
      return "field-name";
    case "CornerRadius":
      return "corner-radius";
    case "LineSpacing":
      return "line-spacing";
  }
}

const cmd = persistenceItemCommandName;

// Swift String equality is canonical equivalence, so names are NFC-normalised for map keys.
const nfc = (s: string) => s.normalize("NFC");

export class IndexedName {
  name: string;
  index: number;

  constructor(name: string, index: number) {
    this.name = name;
    this.index = index;
  }

  get key(): string {
    return `${nfc(this.name)}\0${this.index}`;
  }
}

/** Allow Mapping of element model to tenn and wise verse. */
export function toTenn(self: Element, includeSubElements = true, includeItems = true): TennNode {
  const result = new TennNode("Statements");

  if (self.kind === "Root") {
    // Top-level non-element statements go first; their original interleaving with elements is not kept.
    for (const p of self.properties) {
      result.add(p.clone());
    }
    buildElements(result, self.elements, includeSubElements, includeItems);
  } else {
    buildElements(result, [self], includeSubElements, includeItems);
  }

  return result;
}

export function toTennStr(self: Element, includeSubElements = true, includeItems = true): string {
  const ee = toTenn(self, includeSubElements, includeItems);
  return toStr(ee, 0, false);
}

export function buildItemData(item: DiagramItem, itemBlock: TennNode, addPos: boolean): void {
  if (item.description !== null) {
    itemBlock.add(newCommand(cmd("Description"), newStrNode(item.description)));
  }

  const nx = item.x !== 0;
  const ny = item.y !== 0;

  if (nx || ny || addPos) {
    itemBlock.add(newCommand(cmd("Position"), newFloatNode(item.x), newFloatNode(item.y)));
  }

  for (const p of item.properties) {
    itemBlock.add(p.clone());
  }
}

function buildItem(item: DiagramItem, enodeBlock: TennNode): void {
  const itemRoot = newCommand(cmd("Item"), newStrNode(item.name));

  enodeBlock.add(itemRoot);

  const itemBlock = newBlockExpr();

  buildItemData(item, itemBlock, false);

  if (itemBlock.count > 0) {
    itemRoot.add(itemBlock);
  }
}

export function buildLinkData(item: DiagramItem, linkDataBlock: TennNode, addPos: boolean): void {
  if (item.description !== null) {
    linkDataBlock.add(newCommand(cmd("Description"), newStrNode(item.description)));
  }

  const nx = item.x !== 0;
  const ny = item.y !== 0;

  if (item.name.length > 0) {
    linkDataBlock.add(newCommand(cmd("Label"), newStrNode(item.name)));
  }

  if (nx || ny || addPos) {
    linkDataBlock.add(newCommand(cmd("Position"), newFloatNode(item.x), newFloatNode(item.y)));
  }

  for (const p of item.properties) {
    linkDataBlock.add(p.clone());
  }
}

function buildLink(item: DiagramItem, enodeBlock: TennNode, indexes: Map<DiagramItem, number>): void {
  if (item instanceof LinkItem) {
    const linkCmd = newCommand(cmd("Link"));
    linkCmd.add(newStrNode(item.source !== null ? item.source.name : ""));
    linkCmd.add(newStrNode(item.target !== null ? item.target.name : ""));

    const linkDataBlock = newBlockExpr();

    const sourceIndex = item.source !== null ? indexes.get(item.source) : undefined;
    if (sourceIndex !== undefined && sourceIndex !== 0) {
      linkDataBlock.add(newCommand(cmd("SourceIndex"), newIntNode(sourceIndex)));
    }
    const targetIndex = item.target !== null ? indexes.get(item.target) : undefined;
    if (targetIndex !== undefined && targetIndex !== 0) {
      linkDataBlock.add(newCommand(cmd("TargetIndex"), newIntNode(targetIndex)));
    }

    buildLinkData(item, linkDataBlock, false);

    if (linkDataBlock.count > 0) {
      linkCmd.add(linkDataBlock);
    }

    enodeBlock.add(linkCmd);
  }
}

function buildItems(items: DiagramItem[], enodeBlock: TennNode, itemIndexes: Map<DiagramItem, number>): void {
  for (const item of items) {
    if (item.kind === "Item") {
      buildItem(item, enodeBlock);
    } else if (item.kind === "Link") {
      buildLink(item, enodeBlock, itemIndexes);
    }
  }
}

export function prepareItemRefs(items: DiagramItem[]): Map<DiagramItem, number> {
  // Prepare element index map
  const itemRefNames = new Map<DiagramItem, number>();

  const strToIndex = new Map<string, number>();

  for (const item of items) {
    if (item.kind !== "Item") {
      continue;
    }
    const index = strToIndex.get(nfc(item.name));
    if (index !== undefined) {
      itemRefNames.set(item, index + 1);
      strToIndex.set(nfc(item.name), index + 1);
    } else {
      strToIndex.set(nfc(item.name), 0);
      itemRefNames.set(item, 0);
    }
  }

  return itemRefNames;
}

export function buildElementData(e: Element, enodeBlock: TennNode): void {
  if (e.description !== null && e.description.length > 0) {
    enodeBlock.add(newCommand(cmd("Description"), newStrNode(e.description)));
  }

  for (const p of e.properties) {
    enodeBlock.add(p.clone());
  }
}

function buildElement(e: Element, topParent: TennNode, includeSubElements: boolean, includeItems: boolean): void {
  const enode = newCommand(cmd("Element"), newStrNode(e.name));

  topParent.add(enode);

  const enodeBlock = newBlockExpr();

  enode.add(enodeBlock);

  buildElementData(e, enodeBlock);

  const itemIndexes = prepareItemRefs(e.items);

  if (includeItems) {
    buildItems(e.items, enodeBlock, itemIndexes);
  }

  if (e.elements.length > 0 && includeSubElements) {
    buildElements(enodeBlock, e.elements, includeSubElements, includeItems);
  }
}

function buildElements(topParent: TennNode, elements: Element[], includeSubElements: boolean, includeItems: boolean): void {
  for (const e of elements) {
    buildElement(e, topParent, includeSubElements, includeItems);
  }
}

export function storeItems(items: DiagramItem[]): TennNode {
  const block = new TennNode("Statements");

  const itemIndexes = prepareItemRefs(items);
  buildItems(items, block, itemIndexes);
  return block;
}

/** Parser tenn model into current element state */
export function parseTenn(node: TennNode): ElementModel {
  const result = new ElementModel();
  if (node.kind === "Statements") {
    if (node.children !== null) {
      for (const childElement of node.children) {
        if (childElement.kind === "Command") {
          const el = parseCommand(childElement);
          if (el !== null) {
            result.add(el);
          } else {
            result.properties.append(childElement);
          }
        } else {
          result.properties.append(childElement);
        }
      }
    }
  } else if (node.kind === "Command") {
    const el = parseCommand(node);
    if (el !== null) {
      result.add(el);
    }
  } else {
    result.properties.append(node);
  }
  return result;
}

const blockKinds: TennNodeKind[] = ["Statements", "BlockExpr"];

export function traverseBlock(block: TennNode, visitor: (cmdName: string, node: TennNode) => void): void {
  if (blockKinds.includes(block.kind) && block.children !== null) {
    for (const blChild of block.children) {
      const cmdName = blChild.kind === "Command" && blChild.count > 0 ? blChild.getIdent(0) : null;
      if (cmdName !== null) {
        visitor(cmdName, blChild);
      }
    }
  }
}

function parseChildCommands(node: TennNode, blockIndex: number, visitor: (cmdName: string, child: TennNode) => void): void {
  if (node.count > blockIndex) {
    const block = node.getChild([blockIndex]);
    if (block !== null) {
      traverseBlock(block, visitor);
    }
  }
}

function prepareRefs(items: DiagramItem[]): Map<string, DiagramItem> {
  // Prepare element index map
  const itemRefNames = new Map<string, DiagramItem>();

  const strToIndex = new Map<string, number>();

  for (const item of items) {
    if (item.kind !== "Item") {
      continue;
    }
    const index = strToIndex.get(nfc(item.name));
    if (index !== undefined) {
      itemRefNames.set(new IndexedName(item.name, index + 1).key, item);
      strToIndex.set(nfc(item.name), index + 1);
    } else {
      strToIndex.set(nfc(item.name), 0);
      itemRefNames.set(new IndexedName(item.name, 0).key, item);
    }
  }

  return itemRefNames;
}

export function parseElementData(el: Element, cmdName: string, blChild: TennNode, linkElements: [TennNode, LinkItem][]): void {
  switch (cmdName) {
    case cmd("Item"): {
      const item = parseItem(blChild);
      if (item !== null) {
        el.add(item);
      } else {
        el.properties.append(blChild);
      }
      break;
    }
    case cmd("Link"): {
      const item = parseLink(blChild);
      if (item !== null) {
        el.add(item);
        linkElements.push([blChild, item]);
      } else {
        el.properties.append(blChild);
      }
      break;
    }
    case cmd("Element"): {
      const child = parseElement(blChild);
      if (child !== null) {
        el.add(child);
      } else {
        el.properties.append(blChild);
      }
      break;
    }
    default:
      el.properties.append(blChild);
  }
}

function parseElement(node: TennNode): Element | null {
  const el = new Element("");

  const linkElements: [TennNode, LinkItem][] = [];

  if (node.count >= 2) {
    const name = node.getIdent(1);
    if (name !== null) {
      el.name = name;
    }

    parseChildCommands(node, 2, (cmdName, blChild) => {
      parseElementData(el, cmdName, blChild, linkElements);
    });

    const refs = prepareRefs(el.items);

    for (const [lnode, link] of linkElements) {
      processLink(link, lnode, refs);
      // Keep unresolved links verbatim so a save does not lose their name/index references.
      if (link.source === null || link.target === null) {
        el.items = el.items.filter((i) => i !== link);
        el.properties.append(lnode);
      }
    }

    return el;
  }

  // TODO: need to report error
  return null;
}

export function parseItems(node: TennNode): DiagramItem[] {
  const el = new Element("");
  const linkElements: [TennNode, LinkItem][] = [];

  traverseBlock(node, (cmdName, blChild) => {
    parseElementData(el, cmdName, blChild, linkElements);
  });

  const refs = prepareRefs(el.items);

  for (const [lnode, link] of linkElements) {
    processLink(link, lnode, refs);
  }

  return el.items;
}

export function parseItemData(el: DiagramItem, cmdName: string, blChild: TennNode): void {
  switch (cmdName) {
    case cmd("Position"):
      if (blChild.count === 3) {
        const x = blChild.getFloat(1);
        const y = blChild.getFloat(2);
        if (x !== null && y !== null) {
          el.x = x;
          el.y = y;
        } else {
          el.properties.append(blChild);
        }
      } else {
        el.properties.append(blChild);
      }
      break;
    case cmd("Description"):
    case "desription":
      el.description = blChild.getIdent(1);
      break;
    default:
      el.properties.append(blChild);
  }
}

function parseItem(node: TennNode): DiagramItem | null {
  const el = new DiagramItem("Item", "");

  if (node.count >= 2) {
    const name = node.getIdent(1);
    if (name !== null) {
      el.name = name;
    }

    parseChildCommands(node, 2, (cmdName, blChild) => {
      parseItemData(el, cmdName, blChild);
    });
  }
  return el;
}

function parseLink(node: TennNode): LinkItem | null {
  if (node.count >= 2) {
    return new LinkItem("Link", "", null, null);
  }
  return null;
}

export function parseLinkData(link: DiagramItem, cmdName: string, blChild: TennNode, idx: { source: number; target: number }): void {
  switch (cmdName) {
    case cmd("Description"):
    case "desription":
      link.description = blChild.getIdent(1);
      break;
    case cmd("Label"):
      link.name = blChild.getIdent(1) ?? "";
      break;
    case cmd("Position"):
      if (blChild.count === 3) {
        const x = blChild.getFloat(1);
        const y = blChild.getFloat(2);
        if (x !== null && y !== null) {
          link.x = x;
          link.y = y;
        } else {
          link.properties.append(blChild);
        }
      } else {
        link.properties.append(blChild);
      }
      break;
    case cmd("SourceIndex"): {
      const index = blChild.getInt(1);
      if (index !== null) {
        idx.source = index;
      } else {
        link.properties.append(blChild);
      }
      break;
    }
    case cmd("TargetIndex"): {
      const index = blChild.getInt(1);
      if (index !== null) {
        idx.target = index;
      } else {
        link.properties.append(blChild);
      }
      break;
    }
    default:
      link.properties.append(blChild);
  }
}

function processLink(link: LinkItem, node: TennNode, links: Map<string, DiagramItem>): void {
  const idx = { source: 0, target: 0 };
  parseChildCommands(node, 3, (cmdName, blChild) => {
    parseLinkData(link, cmdName, blChild, idx);
  });

  const source = node.getIdent(1);
  const target = node.getIdent(2);
  if (source !== null && target !== null) {
    const sourceElement = links.get(new IndexedName(source, idx.source).key);
    const targetElement = links.get(new IndexedName(target, idx.target).key);

    if (sourceElement !== undefined) {
      link.source = sourceElement;
    }
    if (targetElement !== undefined) {
      link.target = targetElement;
    }
  }
}

function parseCommand(node: TennNode): Element | null {
  const cmdName = node.getIdent(0);
  if (cmdName !== null) {
    switch (cmdName) {
      case cmd("Element"):
      case cmd("Model"):
        return parseElement(node);
    }
  }
  return null;
}
