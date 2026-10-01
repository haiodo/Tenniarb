// Minimal Foundation UndoManager: only the API ElementModelStore and the app use (registerUndo, grouping, undo/redo, removeAllActions).
// No run loop: groupsByEvent is opt-in and ends the automatic group on a microtask instead of a run-loop turn.
type UndoHandler = () => void;

export class UndoManager {
  private undoStack: UndoHandler[][] = [];
  private redoStack: UndoHandler[][] = [];
  private group: UndoHandler[] = [];
  private level = 0;
  private autoGroup = false;
  groupsByEvent = false;
  isUndoing = false;
  isRedoing = false;

  get canUndo(): boolean {
    return this.undoStack.length > 0;
  }

  get canRedo(): boolean {
    return this.redoStack.length > 0;
  }

  get groupingLevel(): number {
    return this.level;
  }

  beginUndoGrouping(): void {
    if (this.level++ === 0) {
      this.group = [];
    }
  }

  endUndoGrouping(): void {
    if (this.level === 0) {
      throw new Error("endUndoGrouping without beginUndoGrouping");
    }
    if (--this.level === 0 && this.group.length > 0) {
      (this.isUndoing ? this.redoStack : this.undoStack).push(this.group);
    }
  }

  registerUndo(handler: UndoHandler): void {
    // A fresh registration invalidates redo; registrations made while undoing/redoing build the opposite stack.
    if (!this.isUndoing && !this.isRedoing) {
      this.redoStack = [];
    }
    if (this.groupsByEvent && this.level === 0 && !this.isUndoing && !this.isRedoing) {
      this.autoGroup = true;
      this.beginUndoGrouping();
      queueMicrotask(() => this.closeAutoGroup());
    }
    this.beginUndoGrouping();
    this.group.push(handler);
    this.endUndoGrouping();
  }

  private closeAutoGroup(): void {
    if (this.autoGroup) {
      this.autoGroup = false;
      this.endUndoGrouping();
    }
  }

  undo(): void {
    this.closeAutoGroup();
    if (this.level > 0) {
      throw new Error("undo called with an open undo group");
    }
    const group = this.undoStack.pop();
    if (group === undefined) {
      return;
    }
    this.isUndoing = true;
    this.beginUndoGrouping();
    try {
      for (const h of [...group].reverse()) {
        h();
      }
    } finally {
      this.endUndoGrouping();
      this.isUndoing = false;
    }
  }

  redo(): void {
    this.closeAutoGroup();
    if (this.level > 0) {
      throw new Error("redo called with an open undo group");
    }
    const group = this.redoStack.pop();
    if (group === undefined) {
      return;
    }
    this.isRedoing = true;
    this.beginUndoGrouping();
    try {
      for (const h of [...group].reverse()) {
        h();
      }
    } finally {
      this.endUndoGrouping();
      this.isRedoing = false;
    }
  }

  removeAllActions(): void {
    if (this.isUndoing || this.isRedoing) {
      throw new Error("removeAllActions called while undoing or redoing");
    }
    this.autoGroup = false;
    this.undoStack = [];
    this.redoStack = [];
    this.group = [];
    this.level = 0;
  }
}
