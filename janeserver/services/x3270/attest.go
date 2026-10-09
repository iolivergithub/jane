package x3270

// The attest screens: choose an element endpoint, an intent and the rules,
// confirm, and run the attestation the same way the web UI's attest form does.
//
//	step 1  element endpoint   (list, select one)
//	step 2  intent             (list, select one)
//	step 3  rules              (mark any number; Enter with nothing marked continues)
//	step 4  confirm            (operation, message, optional parameters) -> run
//	        result             (the session that was created)

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/racingmars/go3270"

	"a10/structures"
)

// attestChoice accumulates the choices made on the attest screens.
type attestChoice struct {
	Element  structures.Element
	Endpoint string
	Intent   structures.Intent
	Rules    map[string]bool
}

func (c attestChoice) summary() string {
	return fmt.Sprintf("%s (%s) with %s", c.Element.Name, c.Endpoint, c.Intent.Name)
}

func (c attestChoice) ruleNames() []string {
	var rs []string
	for r, on := range c.Rules {
		if on {
			rs = append(rs, r)
		}
	}
	sort.Strings(rs)
	return rs
}

// attestStart is step 1: every endpoint of every element.
func attestStart(s store, home go3270.Tx) go3270.Tx {
	return listTx(listSource{
		Title:  "Attest - step 1 of 4: element endpoint",
		Header: fit("Element", 22) + " " + fit("Endpoint", 10) + " " + fit("Protocol", 14) + " " + "Address",
		Fetch: func(skip, limit int64) ([]listItem, int64, error) {
			es, err := s.Elements()
			if err != nil {
				return nil, 0, err
			}
			sort.SliceStable(es, func(a, b int) bool { return es[a].Name < es[b].Name })
			var all []listItem
			for _, e := range es {
				var eps []string
				for n := range e.Endpoints {
					eps = append(eps, n)
				}
				sort.Strings(eps)
				for _, n := range eps {
					e, n, ep := e, n, e.Endpoints[n]
					all = append(all, listItem{
						Text: fit(e.Name, 22) + " " + fit(n, 10) + " " + fit(ep.Protocol, 14) + " " + ep.Endpoint,
						Open: func(back go3270.Tx) go3270.Tx {
							return attestIntent(s, attestChoice{Element: e, Endpoint: n, Rules: map[string]bool{}}, back, home)
						},
					})
				}
			}
			return page(all, skip, limit), int64(len(all)), nil
		},
	}, home)
}

// attestIntent is step 2.
func attestIntent(s store, c attestChoice, back, home go3270.Tx) go3270.Tx {
	return listTx(listSource{
		Title:  "Attest - step 2 of 4: intent for " + c.Element.Name + " (" + c.Endpoint + ")",
		Header: fit("Intent", 24) + " " + fit("Function", 18) + " " + "Description",
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
					Text: fit(i.Name, 24) + " " + fit(i.Function, 18) + " " + i.Description,
					Open: func(back go3270.Tx) go3270.Tx {
						next := c
						next.Intent = i
						return attestRules(s, next, back, home)
					},
				})
			}
			return items, int64(len(is)), nil
		},
	}, back)
}

// attestRules is step 3: type S beside rules to mark or unmark them; Enter
// with nothing marked goes on to the confirmation.
func attestRules(s store, c attestChoice, back, home go3270.Tx) go3270.Tx {
	const first, rows = 6, lastRow - 6 + 1
	rules := s.Rules()
	sort.SliceStable(rules, func(a, b int) bool { return rules[a].Name < rules[b].Name })
	c.Rules = map[string]bool{}
	top := 0
	message := ""
	var self go3270.Tx
	self = func(conn net.Conn, dev go3270.DevInfo, _ any) (go3270.Tx, any, error) {
		pos := "No rules"
		end := min(top+rows, len(rules))
		if len(rules) > 0 {
			pos = fmt.Sprintf("Row %d to %d of %d", top+1, end, len(rules))
		}
		screen := frame("Attest - step 3 of 4: rules", pos, message, "F3=Back  F7=Up  F8=Down  Enter=Continue  F12=Cancel")
		screen = append(screen,
			go3270.Field{Row: headRow, Col: 0, Content: cut("Attesting "+c.summary(), lineWidth)},
			go3270.Field{Row: headRow + 1, Col: 0, Color: go3270.Blue,
				Content: cut(fmt.Sprintf("Type S to mark or unmark rules (%d marked); Enter with nothing typed continues.", len(c.ruleNames())), lineWidth)},
			go3270.Field{Row: first - 1, Col: rowCol, Intense: true, Content: fit("    Rule", 32) + " " + "Description"})
		for i, r := range rules[top:end] {
			mark := " "
			if c.Rules[r.Name] {
				mark = "*"
			}
			if r.NeedsEV {
				r.Description += " (uses an expected value)"
			}
			screen = append(screen,
				go3270.Field{Row: first + i, Col: selCol, Write: true, Name: fmt.Sprintf("sel%d", i), Highlighting: go3270.Underscore},
				go3270.Field{Row: first + i, Col: rowCol, Intense: c.Rules[r.Name],
					Content: fit(fmt.Sprintf(" %s  %s %s", mark, fit(r.Name, 26), r.Description), rowWidth)})
		}
		message = ""

		resp, err := show(conn, dev, screen)
		if err != nil {
			return nil, nil, err
		}
		switch resp.AID {
		case go3270.AIDPF3:
			return back, nil, nil
		case go3270.AIDPF12:
			return home, nil, nil
		case go3270.AIDPF7:
			top = max(0, top-rows)
			return self, nil, nil
		case go3270.AIDPF8:
			if top+rows < len(rules) {
				top += rows
			} else {
				message = "Already at the end of the list"
			}
			return self, nil, nil
		case go3270.AIDEnter:
		default:
			return self, nil, nil
		}

		marked := false
		for i := range rules[top:end] {
			if strings.TrimSpace(resp.Values[fmt.Sprintf("sel%d", i)]) != "" {
				n := rules[top+i].Name
				c.Rules[n] = !c.Rules[n]
				marked = true
			}
		}
		switch strings.ToUpper(strings.TrimSpace(resp.Values["cmd"])) {
		case "ALL":
			for _, r := range rules {
				c.Rules[r.Name] = true
			}
			marked = true
		case "NONE":
			c.Rules = map[string]bool{}
			marked = true
		}
		if marked {
			return self, nil, nil
		}
		return attestConfirm(s, c, self, home), nil, nil
	}
	return self
}

// attestConfirm is step 4: the operation, message and optional parameters.
// Enter runs the attestation.
func attestConfirm(s store, c attestChoice, back, home go3270.Tx) go3270.Tx {
	op := "AV"
	if len(c.ruleNames()) == 0 {
		op = "AO"
	}
	// fits the 62-character message field
	msg := "Invocation from Jane X3270 initiated " + time.Now().UTC().Format(timeStamp)
	ips, rps := "", ""
	message := ""
	var self go3270.Tx
	self = func(conn net.Conn, dev go3270.DevInfo, _ any) (go3270.Tx, any, error) {
		rules := c.ruleNames()
		ruleText := "(none)"
		if len(rules) > 0 {
			ruleText = fmt.Sprintf("%d: %s", len(rules), strings.Join(rules, ", "))
		}
		ep := c.Element.Endpoints[c.Endpoint]
		screen := frame("Attest - step 4 of 4: confirm", "", message, "F3=Back  Enter=Attest  F12=Cancel")
		screen = append(screen,
			go3270.Field{Row: 3, Col: 0, Content: cut(fit("Element", 17)+c.Element.Name, lineWidth)},
			go3270.Field{Row: 4, Col: 0, Content: cut(fit("Endpoint", 17)+c.Endpoint+"  "+ep.Protocol+"  "+ep.Endpoint, lineWidth)},
			go3270.Field{Row: 5, Col: 0, Content: cut(fit("Intent", 17)+c.Intent.Name+"  ("+c.Intent.Function+")", lineWidth)},
			go3270.Field{Row: 6, Col: 0, Content: cut(fit("Rules", 17)+ruleText, lineWidth)},

			go3270.Field{Row: 8, Col: 0, Intense: true, Content: "Operation  ===>"},
			go3270.Field{Row: 8, Col: 16, Write: true, Name: "op", Content: op, Highlighting: go3270.Underscore, Color: go3270.Turquoise},
			go3270.Field{Row: 8, Col: 19, Color: go3270.Blue, Content: "AV = attest and verify, AO = attest only"},

			go3270.Field{Row: 10, Col: 0, Intense: true, Content: "Message    ===>"},
			go3270.Field{Row: 10, Col: 16, Write: true, Name: "msg", Content: msg, Highlighting: go3270.Underscore, Color: go3270.Turquoise},
			go3270.Field{Row: 10, Col: 79, Autoskip: true},

			go3270.Field{Row: 12, Col: 0, Intense: true, Content: "Intent parameters (JSON object, optional)"},
			go3270.Field{Row: 13, Col: 0, Content: "===>"},
			go3270.Field{Row: 13, Col: 5, Write: true, Name: "ips", Content: ips, Highlighting: go3270.Underscore, Color: go3270.Turquoise},
			go3270.Field{Row: 13, Col: 79, Autoskip: true},

			go3270.Field{Row: 15, Col: 0, Intense: true, Content: "Rule parameters (JSON object, optional; used when verifying)"},
			go3270.Field{Row: 16, Col: 0, Content: "===>"},
			go3270.Field{Row: 16, Col: 5, Write: true, Name: "rps", Content: rps, Highlighting: go3270.Underscore, Color: go3270.Turquoise},
			go3270.Field{Row: 16, Col: 79, Autoskip: true},

			go3270.Field{Row: 19, Col: 0, Color: go3270.Blue, Content: "Press Enter to run the attestation. With AO the rules are not applied."},
		)
		message = ""

		resp, err := go3270.ShowScreenOpts(screen, nil, conn, go3270.ScreenOpts{CursorRow: 8, CursorCol: 17, Codepage: dev.Codepage()})
		if err != nil {
			return nil, nil, err
		}
		op, msg, ips, rps = resp.Values["op"], resp.Values["msg"], resp.Values["ips"], resp.Values["rps"]
		switch resp.AID {
		case go3270.AIDPF3:
			return back, nil, nil
		case go3270.AIDPF12:
			return home, nil, nil
		case go3270.AIDEnter:
		default:
			return self, nil, nil
		}

		req := attestRequest{ElementID: c.Element.ItemID, Endpoint: c.Endpoint, IntentID: c.Intent.ItemID, Message: msg}
		switch strings.ToUpper(strings.TrimSpace(op)) {
		case "AV":
			req.Verify = true
			req.Rules = rules
			if len(rules) == 0 {
				message = "No rules are marked: go back (F3) to mark some, or use AO to attest only"
				return self, nil, nil
			}
		case "AO":
		default:
			message = fmt.Sprintf("Operation must be AV or AO, not %q", clean(op))
			return self, nil, nil
		}
		if req.IntentParams, err = jsonObject(ips); err != nil {
			message = "Intent parameters: " + err.Error()
			return self, nil, nil
		}
		if req.RuleParams, err = jsonObject(rps); err != nil {
			message = "Rule parameters: " + err.Error()
			return self, nil, nil
		}
		return attestRun(s, req, c, home), nil, nil
	}
	return self
}

// jsonObject parses an optional JSON object; blank means none.
func jsonObject(text string) (map[string]interface{}, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		return nil, fmt.Errorf("not a JSON object (%v)", err)
	}
	return m, nil
}

// attestRun shows a "please wait" screen, runs the attestation and then shows
// the session it created. F3 from there returns to the menu.
func attestRun(s store, req attestRequest, c attestChoice, home go3270.Tx) go3270.Tx {
	return func(conn net.Conn, dev go3270.DevInfo, _ any) (go3270.Tx, any, error) {
		what := "Attesting and verifying"
		if !req.Verify {
			what = "Attesting"
		}
		wait := frame("Attest - running", "", "", "Please wait")
		wait = append(wait,
			go3270.Field{Row: 4, Col: 0, Intense: true, Content: cut(what+" "+c.summary(), lineWidth)},
			go3270.Field{Row: 6, Col: 0, Content: "This can take a while; the result will be shown when it is done."})
		if _, err := go3270.ShowScreenOpts(wait, nil, conn, go3270.ScreenOpts{NoResponse: true, CursorRow: cmdRow, CursorCol: cmdCol + 1, Codepage: dev.Codepage()}); err != nil {
			return nil, nil, err
		}

		sid, aerr := s.Attest(req)

		var d detail
		if sid != "" {
			d = sessionDetail(s, sid)
		}
		// what was attested comes first; the element and intent lines open them
		var head detail
		if aerr != nil {
			head.add(wrap("Attestation did not complete: "+aerr.Error(), lineWidth))
		} else {
			head.add([]string{"Attestation complete."})
		}
		head.add([]string{""})
		head.add(labelled("Element", c.Element.Name+"  ("+c.Endpoint+")"),
			link{Open: func() detail { return elementDetail(s, c.Element.ItemID) }})
		head.add(labelled("Intent", c.Intent.Name),
			link{Open: func() detail { return intentDetail(s, c.Intent) }})
		head.add([]string{""})
		d.prepend(head.Lines, head.Links)
		d.Title = "Attest - result"
		d.Err = aerr
		return textTx(d, home), nil, nil
	}
}
