package x3270

import (
	"a10/registernames"

	"github.com/racingmars/go3270"
)

// Register naming schemes (#101): built into Jane, read only.

func pcrNamesList(s store, back go3270.Tx) go3270.Tx {
	return listTx(listSource{
		Title:  "Register names",
		Header: fit("Scheme", 20) + " " + fit("Platform", 8) + " " + "Title",
		Fetch: func(skip, limit int64) ([]listItem, int64, error) {
			all := registernames.Schemes()
			var items []listItem
			for _, sc := range page(all, skip, limit) {
				sc := sc
				items = append(items, listItem{
					Text:   fit(sc.Key, 20) + " " + fit(sc.Platform, 8) + " " + sc.Title,
					Detail: func() detail { return pcrSchemeDetail(sc) },
				})
			}
			return items, int64(len(all)), nil
		},
	}, back)
}

func pcrSchemeDetail(sc registernames.Scheme) detail {
	d := detail{Title: "Register names " + sc.Key}
	add := func(l ...string) { d.Lines = append(d.Lines, l...) }
	add(labelled("Title", sc.Title)...)
	add(labelled("Platform", sc.Platform)...)
	add(labelled("Description", sc.Description)...)
	add(labelled("Reference", sc.Reference)...)
	if sc.Key == registernames.Default {
		add("(the default scheme)")
	}
	add(section("Registers")...)
	for _, r := range sc.Registers {
		add(labelled(r.Register, r.Short)...)
		add(labelled("", r.Long)...)
	}
	return d
}
