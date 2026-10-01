// Port of ElementProperties.swift. Swift extension methods on Element/DiagramItem are free functions here.
import { newCommand, newNode, newStrNode } from "./model.ts";
import type { TennNode, TennNodeKind } from "./model.ts";
import { TennParser } from "./parser.ts";
import { toStr } from "./printer.ts";
import { DiagramItem, Element, LinkItem, ModelProperties } from "./element-model.ts";
import {
  buildElementData,
  buildItemData,
  buildLinkData,
  parseElementData,
  parseItemData,
  parseLinkData,
  persistenceItemCommandName,
  traverseBlock,
} from "./persistence.ts";

/** Convert items to list of properties */
export function toTennAsProps(self: Element | DiagramItem, kind: TennNodeKind = "Statements", reparse = false): TennNode {
  const items = newNode(kind);

  if (self instanceof Element) {
    items.add(newCommand("name", newStrNode(self.name)));
    buildElementData(self, items);
  } else if (self.kind === "Item") {
    if (self.name.length > 0) {
      items.add(newCommand(persistenceItemCommandName("Name"), newStrNode(self.name)));
    }
    buildItemData(self, items, true);
  } else if (self.kind === "Link") {
    buildLinkData(self, items, true);
  }

  // We need to convert it to/back to have a proper positioning
  if (reparse) {
    return new TennParser().parse(toStr(items, 0, false));
  }
  return items;
}

export function fromTennProps(self: Element | DiagramItem, node: TennNode): void {
  if (self instanceof Element) {
    self.properties = new ModelProperties();

    const linkElements: [TennNode, LinkItem][] = [];
    traverseBlock(node, (cmdName, blChild) => {
      if (cmdName === "name") {
        const newName = blChild.getIdent(1);
        if (newName !== null) {
          self.name = newName;
        }
        return;
      }
      parseElementData(self, cmdName, blChild, linkElements);
    });
  } else if (self.kind === "Item") {
    self.properties = new ModelProperties();
    self.x = 0; // In case pos was deleted
    self.y = 0;
    traverseBlock(node, (cmdName, blChild) => {
      if (cmdName === persistenceItemCommandName("Name")) {
        const newName = blChild.getIdent(1);
        if (newName !== null) {
          self.name = newName;
        }
        return;
      }
      parseItemData(self, cmdName, blChild);
    });
  } else if (self.kind === "Link") {
    const idx = { source: 0, target: 0 };
    self.properties = new ModelProperties();
    self.x = 0; // In case pos was deleted
    self.y = 0;
    self.name = "";
    traverseBlock(node, (cmdName, blChild) => {
      if (cmdName === persistenceItemCommandName("Label")) {
        const newName = blChild.getIdent(1);
        if (newName !== null) {
          self.name = newName;
        }
        return;
      }
      parseLinkData(self, cmdName, blChild, idx);
    });
  }
}
