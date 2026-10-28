package model

import "testing"

func TestUndoManagerDropsOldestAboveMaxGroups(t *testing.T) {
	um := &UndoManager{MaxGroups: 3}
	var undone []int
	for i := 1; i <= 5; i++ {
		um.RegisterUndo(func() error { undone = append(undone, i); return nil })
		um.CloseAutoGroup()
	}
	for um.CanUndo() {
		if err := um.Undo(); err != nil {
			t.Fatal(err)
		}
	}
	if len(undone) != 3 || undone[0] != 5 || undone[2] != 3 {
		t.Errorf("undone %v, want [5 4 3]", undone)
	}
}
