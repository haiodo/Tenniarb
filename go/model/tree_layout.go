package model

// TreeLayout is an empty stub in Swift too (its direction/bounds/leftSize/layerSize fields are unused there).
type TreeLayout struct{}

func (TreeLayout) Apply(_ *LayoutContext, _ bool) ([]ElementOperation, error) {
	return nil, nil
}
