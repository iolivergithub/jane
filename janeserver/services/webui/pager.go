package webui

import (
	"fmt"
	"strconv"
)

// listPageSize is how many rows the paged list pages (sessions, claims,
// results) show at a time.
const listPageSize = 100

// pager describes one page of a long list and the links to the others.
type pager struct {
	Page, Pages int   // 1-based current page and number of pages
	Total       int64 // total number of items
	From, To    int64 // 1-based range of items on this page (0, 0 when empty)
	Skip, Size  int64 // what to ask the database for
	Links       []pagerLink
}

type pagerLink struct {
	Label    string
	Href     string
	Active   bool
	Disabled bool
}

// newPager works out which page is wanted from the ?page= value (anything
// invalid means page 1; past the end means the last page) and builds the links
// for path. When the total is unknown (negative) it is treated as empty.
func newPager(pageParam string, total, size int64, path string) pager {
	if total < 0 {
		total = 0
	}
	pages := int((total + size - 1) / size)
	if pages < 1 {
		pages = 1
	}
	page, err := strconv.Atoi(pageParam)
	if err != nil || page < 1 {
		page = 1
	}
	if page > pages {
		page = pages
	}

	p := pager{Page: page, Pages: pages, Total: total, Skip: int64(page-1) * size, Size: size}
	if total > 0 {
		p.From = p.Skip + 1
		p.To = min(p.Skip+size, total)
	}

	if pages > 1 {
		href := func(n int) string { return fmt.Sprintf("%s?page=%d", path, n) }
		p.Links = append(p.Links, pagerLink{Label: "«", Href: href(page - 1), Disabled: page == 1})
		last := 0
		for n := 1; n <= pages; n++ {
			// first, last and two either side of the current page
			if n != 1 && n != pages && (n < page-2 || n > page+2) {
				continue
			}
			if last != 0 && n > last+1 {
				p.Links = append(p.Links, pagerLink{Label: "…", Disabled: true})
			}
			p.Links = append(p.Links, pagerLink{Label: strconv.Itoa(n), Href: href(n), Active: n == page})
			last = n
		}
		p.Links = append(p.Links, pagerLink{Label: "»", Href: href(page + 1), Disabled: page == pages})
	}
	return p
}
