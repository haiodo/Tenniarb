package editor

import (
	"slices"
	"strings"

	"github.com/haiodo/tenniarb/go/model"
)

// SearchItems are the items of element whose name or body contains query (case-insensitive), by
// name. An empty query matches all. This is the list of the Goto Item popup (Swift
// SearchBoxViewController).
func SearchItems(element *model.Element, query string) []*model.DiagramItem {
	q := strings.ToLower(query)
	var out []*model.DiagramItem
	for _, i := range element.Items {
		if strings.Contains(strings.ToLower(i.Name), q) || strings.Contains(strings.ToLower(BodyText(i)), q) {
			out = append(out, i)
		}
	}
	slices.SortStableFunc(out, func(a, b *model.DiagramItem) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// RowText is the text of a result row: "name - body", newlines shown as \n.
func RowText(item *model.DiagramItem) string {
	text := item.Name
	if body := BodyText(item); body != "" {
		text += " - " + body
	}
	return strings.ReplaceAll(text, "\n", `\n`)
}
