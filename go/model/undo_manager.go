package model

import (
	"errors"
	"slices"
)

// UndoHandler may fail: an undo step re-executes a store operation, which reports a bad index as an error.
type UndoHandler func() error

// UndoManager is a port of web/packages/core/src/undo-manager.ts, a minimal Foundation UndoManager.
// The TS one closes the automatic group (GroupsByEvent) on a microtask; Go has no run loop, so the host
// calls CloseAutoGroup at the end of each event. Undo and Redo close it first, as in TS.
// Where TS throws, the methods here return an error.
type UndoManager struct {
	undoStack [][]UndoHandler
	redoStack [][]UndoHandler
	group     []UndoHandler
	level     int
	autoGroup bool

	GroupsByEvent bool
	IsUndoing     bool
	IsRedoing     bool
}

func (u *UndoManager) CanUndo() bool { return len(u.undoStack) > 0 }

func (u *UndoManager) CanRedo() bool { return len(u.redoStack) > 0 }

func (u *UndoManager) GroupingLevel() int { return u.level }

func (u *UndoManager) BeginUndoGrouping() {
	if u.level == 0 {
		u.group = nil
	}
	u.level++
}

func (u *UndoManager) EndUndoGrouping() error {
	if u.level == 0 {
		return errors.New("endUndoGrouping without beginUndoGrouping")
	}
	u.level--
	if u.level == 0 && len(u.group) > 0 {
		if u.IsUndoing {
			u.redoStack = append(u.redoStack, u.group)
		} else {
			u.undoStack = append(u.undoStack, u.group)
		}
	}
	return nil
}

func (u *UndoManager) RegisterUndo(handler UndoHandler) {
	// A fresh registration invalidates redo; registrations made while undoing/redoing build the opposite stack.
	if !u.IsUndoing && !u.IsRedoing {
		u.redoStack = nil
	}
	if u.GroupsByEvent && u.level == 0 && !u.IsUndoing && !u.IsRedoing {
		u.autoGroup = true
		u.BeginUndoGrouping()
	}
	u.BeginUndoGrouping()
	u.group = append(u.group, handler)
	_ = u.EndUndoGrouping() // level was raised just above
}

// CloseAutoGroup ends the group opened by RegisterUndo when GroupsByEvent is set.
func (u *UndoManager) CloseAutoGroup() {
	if u.autoGroup {
		u.autoGroup = false
		_ = u.EndUndoGrouping() // autoGroup implies an open level
	}
}

func (u *UndoManager) Undo() error {
	u.CloseAutoGroup()
	if u.level > 0 {
		return errors.New("undo called with an open undo group")
	}
	if len(u.undoStack) == 0 {
		return nil
	}
	group := u.undoStack[len(u.undoStack)-1]
	u.undoStack = u.undoStack[:len(u.undoStack)-1]

	u.IsUndoing = true
	u.BeginUndoGrouping()
	err := runReversed(group)
	err = errors.Join(err, u.EndUndoGrouping())
	u.IsUndoing = false
	return err
}

func (u *UndoManager) Redo() error {
	u.CloseAutoGroup()
	if u.level > 0 {
		return errors.New("redo called with an open undo group")
	}
	if len(u.redoStack) == 0 {
		return nil
	}
	group := u.redoStack[len(u.redoStack)-1]
	u.redoStack = u.redoStack[:len(u.redoStack)-1]

	u.IsRedoing = true
	u.BeginUndoGrouping()
	err := runReversed(group)
	err = errors.Join(err, u.EndUndoGrouping())
	u.IsRedoing = false
	return err
}

func runReversed(group []UndoHandler) error {
	for _, h := range slices.Backward(slices.Clone(group)) {
		if err := h(); err != nil {
			return err
		}
	}
	return nil
}

func (u *UndoManager) RemoveAllActions() error {
	if u.IsUndoing || u.IsRedoing {
		return errors.New("removeAllActions called while undoing or redoing")
	}
	u.autoGroup = false
	u.undoStack = nil
	u.redoStack = nil
	u.group = nil
	u.level = 0
	return nil
}
