package seatlayer

import (
	"context"
	"iter"
)

// Page is one page of a list endpoint, plus the cursor for the next.
type Page struct {
	// Items are the rows on this page.
	Items []map[string]any
	// NextCursor is empty once the list is exhausted.
	NextCursor string
}

func pageFrom(response map[string]any, key string) Page {
	page := Page{}
	if raw, ok := response[key].([]any); ok {
		page.Items = make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			if m, ok := item.(map[string]any); ok {
				page.Items = append(page.Items, m)
			}
		}
	}
	if cursor, ok := response["nextCursor"].(string); ok {
		page.NextCursor = cursor
	}
	return page
}

// paginate walks every page lazily.
//
// A range-over-func iterator rather than a materialised slice: the point of
// paginating was to stop holding an unbounded result set in memory, and
// returning []map[string]any would hand that problem straight back. Errors ride
// alongside each item so a caller cannot accidentally ignore a failed page:
//
//	for chart, err := range client.Charts.All(ctx, nil) {
//		if err != nil {
//			return err
//		}
//		...
//	}
func paginate(
	ctx context.Context,
	fetch func(ctx context.Context, cursor string) (Page, error),
) iter.Seq2[map[string]any, error] {
	return func(yield func(map[string]any, error) bool) {
		cursor := ""
		for {
			page, err := fetch(ctx, cursor)
			if err != nil {
				yield(nil, err)
				return
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
			// An empty cursor terminates, so a caller looping cannot spin forever.
			if page.NextCursor == "" {
				return
			}
			cursor = page.NextCursor
		}
	}
}
