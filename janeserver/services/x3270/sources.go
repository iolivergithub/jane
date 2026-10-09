package x3270

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/racingmars/go3270"

	"a10/structures"
)

// page returns the part of a fully loaded list that a page shows.
func page[T any](all []T, skip, limit int64) []T {
	if skip >= int64(len(all)) {
		return nil
	}
	return all[skip:min(skip+limit, int64(len(all)))]
}

// names maps element IDs to element names (for lists that show the element).
func names(s store) map[string]string {
	m := map[string]string{}
	if es, err := s.Elements(); err == nil {
		for _, e := range es {
			m[e.ItemID] = e.Name
		}
	}
	return m
}

func nameOr(m map[string]string, id string) string {
	if n := m[id]; n != "" {
		return n
	}
	return id
}

func countResults(rs []structures.Result) (pass, fail, other int) {
	for _, r := range rs {
		switch r.Result {
		case structures.Success:
			pass++
		case structures.Fail:
			fail++
		default:
			other++
		}
	}
	return
}

func countClaimErrors(cs []structures.Claim) int {
	n := 0
	for _, c := range cs {
		if c.BodyType == structures.CLAIMERROR {
			n++
		}
	}
	return n
}

func claimIntent(c structures.Claim) string {
	if c.Header.Intent.Name != "" {
		return c.Header.Intent.Name
	}
	return c.BodyType
}

func claimType(c structures.Claim) string {
	if c.BodyType == structures.CLAIMERROR {
		return "ERROR"
	}
	return c.BodyType
}

func section(title string) []string {
	return []string{"", title, strings.Repeat("-", len(title))}
}

// ---------------------------------------------------------------- elements

func elementsList(s store, back go3270.Tx) go3270.Tx {
	return listTx(listSource{
		Title:  "Elements",
		Header: fit("Name", 24) + " " + fit("Host", 20) + " " + "Endpoints / tags",
		Fetch: func(skip, limit int64) ([]listItem, int64, error) {
			es, err := s.Elements()
			if err != nil {
				return nil, 0, err
			}
			sort.SliceStable(es, func(a, b int) bool { return es[a].Name < es[b].Name })
			var items []listItem
			for _, e := range page(es, skip, limit) {
				e := e
				var eps []string
				for n := range e.Endpoints {
					eps = append(eps, n)
				}
				sort.Strings(eps)
				extra := strings.Join(eps, ",")
				if len(e.Tags) > 0 {
					extra += "  [" + strings.Join(e.Tags, ",") + "]"
				}
				items = append(items, listItem{
					Text:   fit(e.Name, 24) + " " + fit(e.Host.Hostname, 20) + " " + extra,
					Detail: func() detail { return elementDetail(s, e.ItemID) },
				})
			}
			return items, int64(len(es)), nil
		},
	}, back)
}

func elementDetail(s store, id string) detail {
	e, err := s.Element(id)
	if err != nil {
		return detail{Title: "Element", Err: fmt.Errorf("could not read element %s: %v", id, err)}
	}
	d := detail{Title: "Element " + e.Name}
	add := func(l ...string) { d.Lines = append(d.Lines, l...) }

	add(labelled("Name", e.Name)...)
	add(labelled("Item ID", e.ItemID)...)
	add(labelled("Description", e.Description)...)
	add(labelled("Tags", strings.Join(e.Tags, ", "))...)

	add(section("Endpoints")...)
	var eps []string
	for n := range e.Endpoints {
		eps = append(eps, n)
	}
	sort.Strings(eps)
	for _, n := range eps {
		add(labelled(n, e.Endpoints[n].Protocol+"  "+e.Endpoints[n].Endpoint)...)
	}
	if len(eps) == 0 {
		add("(none)")
	}

	add(section("Host")...)
	add(labelled("Hostname", e.Host.Hostname)...)
	add(labelled("OS / arch", strings.Trim(e.Host.OS+" / "+e.Host.Arch, " /"))...)
	add(labelled("Machine ID", e.Host.MachineID)...)

	add(section("TPM2")...)
	if e.TPM2.Device == "" {
		add("(no TPM device)")
	} else {
		add(labelled("Device", e.TPM2.Device)...)
		add(labelled("EK cert handle", e.TPM2.EKCertHandle)...)
		add(labelled("EK handle", e.TPM2.EK.Handle)...)
		add(labelled("AK handle", e.TPM2.AK.Handle)...)
	}

	add(section("Logs")...)
	add(labelled("UEFI event log", or(e.UEFI.Eventlog, "(not specified)"))...)
	add(labelled("IMA log", or(e.IMA.ASCIILog, "(not specified)"))...)
	add(labelled("TXT log", or(e.TXT.Log, "(not specified)"))...)

	add(section("Latest results")...)
	rs, rerr := s.ResultsByElement(e.ItemID, 20)
	switch {
	case rerr != nil:
		add("(could not read results: " + rerr.Error() + ")")
	case len(rs) == 0:
		add("(none)")
	default:
		for _, r := range rs {
			add(fmt.Sprintf("%s  %s %s", when(r.VerifiedAt), fit(r.RuleName, 30), resultLabel(r.Result)))
		}
	}
	return d
}

func or(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

// ---------------------------------------------------------------- intents

func intentsList(s store, back go3270.Tx) go3270.Tx {
	return listTx(listSource{
		Title:  "Intents",
		Header: fit("Name", 24) + " " + fit("Function", 18) + " " + "Description",
		Fetch: func(skip, limit int64) ([]listItem, int64, error) {
			is, err := s.Intents()
			if err != nil {
				return nil, 0, err
			}
			sort.SliceStable(is, func(a, b int) bool { return is[a].Name < is[b].Name })
			var items []listItem
			for _, i := range page(is, skip, limit) {
				i := i
				items = append(items, listItem{
					Text:   fit(i.Name, 24) + " " + fit(i.Function, 18) + " " + i.Description,
					Detail: func() detail { return intentDetail(s, i) },
				})
			}
			return items, int64(len(is)), nil
		},
	}, back)
}

func intentDetail(s store, i structures.Intent) detail {
	d := detail{Title: "Intent " + i.Name}
	add := func(l ...string) { d.Lines = append(d.Lines, l...) }
	add(labelled("Name", i.Name)...)
	add(labelled("Item ID", i.ItemID)...)
	add(labelled("Function", i.Function)...)
	add(labelled("Description", i.Description)...)
	add(section("Parameters")...)
	if len(i.Parameters) == 0 {
		add("(none)")
	} else {
		add(flatten("", i.Parameters)...)
	}

	add(section("Expected values using this intent")...)
	evs, err := s.ExpectedValues()
	if err != nil {
		add("(could not read expected values: " + err.Error() + ")")
		return d
	}
	el := names(s)
	n := 0
	for _, ev := range evs {
		if ev.IntentID == i.ItemID {
			add(fmt.Sprintf("%s %s %s", fit(nameOr(el, ev.ElementID), 24), fit(ev.EndpointName, 12), ev.Name))
			n++
		}
	}
	if n == 0 {
		add("(none)")
	}
	return d
}

// ---------------------------------------------------------------- expected values

func evsList(s store, back go3270.Tx) go3270.Tx {
	return listTx(listSource{
		Title:  "Expected Values",
		Header: fit("Name", 26) + " " + fit("Element", 20) + " " + fit("Intent", 18) + " " + "Endpoint",
		Fetch: func(skip, limit int64) ([]listItem, int64, error) {
			evs, err := s.ExpectedValues()
			if err != nil {
				return nil, 0, err
			}
			el := names(s)
			in := map[string]string{}
			if is, ierr := s.Intents(); ierr == nil {
				for _, i := range is {
					in[i.ItemID] = i.Name
				}
			}
			sort.SliceStable(evs, func(a, b int) bool {
				ea, eb := nameOr(el, evs[a].ElementID), nameOr(el, evs[b].ElementID)
				if ea != eb {
					return ea < eb
				}
				return evs[a].Name < evs[b].Name
			})
			var items []listItem
			for _, ev := range page(evs, skip, limit) {
				ev := ev
				items = append(items, listItem{
					Text:   fit(ev.Name, 26) + " " + fit(nameOr(el, ev.ElementID), 20) + " " + fit(nameOr(in, ev.IntentID), 18) + " " + ev.EndpointName,
					Detail: func() detail { return evDetail(s, ev, nameOr(el, ev.ElementID), nameOr(in, ev.IntentID)) },
				})
			}
			return items, int64(len(evs)), nil
		},
	}, back)
}

func evDetail(s store, ev structures.ExpectedValue, element, intent string) detail {
	d := detail{Title: "Expected Value " + ev.Name}
	add := func(l ...string) { d.Lines = append(d.Lines, l...) }
	add(labelled("Name", ev.Name)...)
	add(labelled("Item ID", ev.ItemID)...)
	add(labelled("Description", ev.Description)...)
	add(labelled("Element", element)...)
	add(labelled("Endpoint", ev.EndpointName)...)
	add(labelled("Intent", intent)...)
	add(section("Values")...)
	if len(ev.EVS) == 0 {
		add("(none)")
	} else {
		add(flatten("", ev.EVS)...)
	}
	if ev.RecordHistory.Created != 0 {
		add(section("Record history")...)
		add(labelled("Created", when(ev.RecordHistory.Created))...)
		if ev.RecordHistory.LastUpdated != 0 {
			add(labelled("Last updated", when(ev.RecordHistory.LastUpdated))...)
		}
		if ev.RecordHistory.Archived != 0 {
			add(labelled("Archived", when(ev.RecordHistory.Archived)+"  "+ev.RecordHistory.Reason)...)
		}
	}
	return d
}

// ---------------------------------------------------------------- sessions

func sessionsList(s store, back go3270.Tx) go3270.Tx {
	return listTx(listSource{
		Title:  "Sessions",
		Header: fit("Opened (UTC)", 19) + " " + fit("Took", 9) + " " + fit("Claims", 6) + " " + fit("Results", 7) + " " + "Message",
		Fetch: func(skip, limit int64) ([]listItem, int64, error) {
			ss, err := s.SessionsPage(skip, limit)
			if err != nil {
				return nil, 0, err
			}
			var items []listItem
			for _, x := range ss {
				x := x
				dur := took(x.Timing.Opened, x.Timing.Closed)
				if x.Timing.Closed == 0 {
					dur = "open"
				}
				items = append(items, listItem{
					Text: fmt.Sprintf("%s %s %6d %7d %s", when(x.Timing.Opened), fit(dur, 9),
						len(x.ClaimList), len(x.ResultList), x.Message),
					Detail: func() detail { return sessionDetail(s, x.ItemID) },
				})
			}
			return items, s.CountSessions(), nil
		},
	}, back)
}

func sessionDetail(s store, id string) detail {
	x, err := s.Session(id)
	if err != nil {
		return detail{Title: "Session", Err: fmt.Errorf("could not read session %s: %v", id, err)}
	}
	cs, cerr := s.ClaimsBySession(id)
	rs, rerr := s.ResultsBySession(id)
	el := names(s)

	d := detail{Title: "Session " + when(x.Timing.Opened)}
	add := func(l ...string) { d.Lines = append(d.Lines, l...) }
	add(labelled("Item ID", x.ItemID)...)
	add(labelled("Opened", when(x.Timing.Opened))...)
	if x.Timing.Closed == 0 {
		add(labelled("Closed", "(still open)")...)
	} else {
		add(labelled("Closed", when(x.Timing.Closed))...)
		add(labelled("Took", took(x.Timing.Opened, x.Timing.Closed))...)
	}
	add(labelled("Message", x.Message)...)
	p, f, o := countResults(rs)
	add(labelled("Results", fmt.Sprintf("%d pass, %d fail, %d other", p, f, o))...)
	add(labelled("Claims", fmt.Sprintf("%d (%d errors)", len(cs), countClaimErrors(cs)))...)

	add(section("Results")...)
	switch {
	case rerr != nil:
		add("(could not read results: " + rerr.Error() + ")")
	case len(rs) == 0:
		add("(none)")
	default:
		sort.SliceStable(rs, func(a, b int) bool {
			ea, eb := nameOr(el, rs[a].ElementID), nameOr(el, rs[b].ElementID)
			if ea != eb {
				return ea < eb
			}
			return rs[a].RuleName < rs[b].RuleName
		})
		for _, r := range rs {
			add(fmt.Sprintf("%s %s %s %s", fit(nameOr(el, r.ElementID), 18), fit(r.RuleName, 26), fit(resultLabel(r.Result), 12), r.Message))
		}
	}

	add(section("Claims")...)
	switch {
	case cerr != nil:
		add("(could not read claims: " + cerr.Error() + ")")
	case len(cs) == 0:
		add("(none)")
	default:
		for _, c := range cs {
			add(fmt.Sprintf("%s %s %s %s", fit(c.Header.Element.Name, 18), fit(claimIntent(c), 22), fit(claimType(c), 16),
				took(c.Header.Timing.Requested, c.Header.Timing.Received)))
		}
	}
	return d
}

// ---------------------------------------------------------------- claims

func claimsList(s store, back go3270.Tx) go3270.Tx {
	return listTx(listSource{
		Title:  "Claims",
		Header: fit("Requested (UTC)", 19) + " " + fit("Element", 18) + " " + fit("Intent", 20) + " " + "Type",
		Fetch: func(skip, limit int64) ([]listItem, int64, error) {
			cs, err := s.ClaimsPage(skip, limit)
			if err != nil {
				return nil, 0, err
			}
			var items []listItem
			for _, c := range cs {
				c := c
				it := listItem{
					Text:   fmt.Sprintf("%s %s %s %s", when(c.Header.Timing.Requested), fit(c.Header.Element.Name, 18), fit(claimIntent(c), 20), claimType(c)),
					Detail: func() detail { return claimDetail(s, c.ItemID) },
				}
				if c.BodyType == structures.CLAIMERROR {
					it.Color = go3270.Red
				}
				items = append(items, it)
			}
			return items, s.CountClaims(), nil
		},
	}, back)
}

// maxBodyLines keeps a very large claim body (e.g. a long IMA log) from
// producing an unreasonably long screen.
const maxBodyLines = 2000

func claimDetail(s store, id string) detail {
	c, err := s.Claim(id)
	if err != nil {
		return detail{Title: "Claim", Err: fmt.Errorf("could not read claim %s: %v", id, err)}
	}
	d := detail{Title: "Claim " + claimIntent(c)}
	add := func(l ...string) { d.Lines = append(d.Lines, l...) }
	add(labelled("Item ID", c.ItemID)...)
	add(labelled("Type", claimType(c))...)
	add(labelled("Element", c.Header.Element.Name)...)
	add(labelled("Endpoint", c.Header.EndpointName+"  "+c.Header.Endpoint.Protocol+"  "+c.Header.Endpoint.Endpoint)...)
	add(labelled("Intent", claimIntent(c))...)
	add(labelled("Session", c.Header.Session.ItemID)...)
	add(labelled("Requested", when(c.Header.Timing.Requested))...)
	add(labelled("Received", when(c.Header.Timing.Received))...)
	add(labelled("Took", took(c.Header.Timing.Requested, c.Header.Timing.Received))...)

	add(section("Body")...)
	var body []string
	if c.BodyType == "ima/asciilog" {
		body = imaBody(c.Body)
	}
	if body == nil {
		if len(c.Body) == 0 {
			body = []string{"(empty)"}
		} else {
			body = flatten("", c.Body)
		}
	}
	if len(body) > maxBodyLines {
		body = append(body[:maxBodyLines], fmt.Sprintf("... %d more lines not shown", len(body)-maxBodyLines))
	}
	add(body...)
	return d
}

// imaBody decodes the IMA log in an ima/asciilog claim, one line per entry.
func imaBody(b map[string]interface{}) []string {
	enc, ok := b["asciilog"].(string)
	if !ok {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil
	}
	out := []string{fmt.Sprintf("%v %v bytes encoded, %v bytes original", b["encoded"], b["encodedlength"], b["unencodedlength"]), ""}
	for _, l := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		// an entry is longer than a line: indent its continuation lines so
		// each entry starts at the left margin
		w := wrap(l, lineWidth)
		out = append(out, w[0])
		for _, c := range wrap(strings.Join(w[1:], " "), lineWidth-4) {
			if c != "" {
				out = append(out, "    "+c)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------- results

func resultsList(s store, back go3270.Tx) go3270.Tx {
	return listTx(listSource{
		Title:  "Results",
		Header: fit("Verified (UTC)", 19) + " " + fit("Element", 18) + " " + fit("Rule", 24) + " " + "Result",
		Fetch: func(skip, limit int64) ([]listItem, int64, error) {
			rs, err := s.ResultsPage(skip, limit)
			if err != nil {
				return nil, 0, err
			}
			el := names(s)
			var items []listItem
			for _, r := range rs {
				r := r
				items = append(items, listItem{
					Text:   fmt.Sprintf("%s %s %s %s", when(r.VerifiedAt), fit(nameOr(el, r.ElementID), 18), fit(r.RuleName, 24), resultLabel(r.Result)),
					Color:  resultColor(r.Result),
					Detail: func() detail { return resultDetail(s, r.ItemID) },
				})
			}
			return items, s.CountResults(), nil
		},
	}, back)
}

func resultDetail(s store, id string) detail {
	r, err := s.Result(id)
	if err != nil {
		return detail{Title: "Result", Err: fmt.Errorf("could not read result %s: %v", id, err)}
	}
	d := detail{Title: "Result " + r.RuleName}
	add := func(l ...string) { d.Lines = append(d.Lines, l...) }
	add(labelled("Rule", r.RuleName)...)
	add(labelled("Result", resultLabel(r.Result))...)
	add(labelled("Message", r.Message)...)
	add(labelled("Verified", when(r.VerifiedAt))...)
	add(labelled("Item ID", r.ItemID)...)

	element := r.ElementID
	if e, eerr := s.Element(r.ElementID); eerr == nil && e.Name != "" {
		element = e.Name
	}
	add(labelled("Element", element)...)

	ev := r.ExpectedValue
	if r.ExpectedValue.ItemID != "" {
		if cur, everr := s.ExpectedValue(r.ExpectedValue.ItemID); everr == nil {
			ev = cur
		}
	}
	intent := ev.IntentID
	if ev.IntentID != "" {
		if i, ierr := s.Intent(ev.IntentID); ierr == nil && i.Name != "" {
			intent = i.Name
		}
	}
	add(labelled("Intent", intent)...)
	add(labelled("Expected value", or(ev.Name, or(ev.ItemID, "(none)")))...)
	add(labelled("Claim", r.ClaimID)...)
	add(labelled("Session", r.Session.ItemID)...)

	add(section("Parameters")...)
	if len(r.Parameters) == 0 {
		add("(none)")
	} else {
		add(flatten("", r.Parameters)...)
	}
	if len(ev.EVS) > 0 {
		add(section("Expected value")...)
		add(flatten("", ev.EVS)...)
	}
	return d
}
