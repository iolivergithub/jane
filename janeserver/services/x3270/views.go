package x3270

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/racingmars/go3270"
)

// show sends a screen and waits for the user, keeping the cursor in the
// command field.
func show(conn net.Conn, dev go3270.DevInfo, screen go3270.Screen) (go3270.Response, error) {
	return send(conn, dev, screen, go3270.ScreenOpts{
		CursorRow: cmdRow, CursorCol: cmdCol + 1, Codepage: dev.Codepage(),
	})
}

func isBack(aid go3270.AID) bool { return aid == go3270.AIDPF3 || aid == go3270.AIDPF12 }

// ---------------------------------------------------------------- menu

type menuItem struct {
	Key, Label string
	Open       func(s store, back go3270.Tx) go3270.Tx
}

var menuItems = []menuItem{
	{"1", "Elements          attested machines and their endpoints", elementsList},
	{"2", "Intents           what to ask an element for", intentsList},
	{"3", "Expected values   reference values to check claims against", evsList},
	{"4", "Sessions          attestation sessions, newest first", sessionsList},
	{"5", "Claims            evidence collected from elements, newest first", claimsList},
	{"6", "Results           outcomes of applying rules to claims, newest first", resultsList},
	{"7", "Attest            choose an element, intent and rules, and attest", attestStart},
	{"8", "Log               Jane's log, newest first", logList},
	{"9", "Configuration     this attestation server's settings and record counts", configScreen},
	{"10", "Register names    names of TPM PCRs and confidential computing registers", pcrNamesList},
}

// menuAction is what a command typed on the primary option menu does.
type menuAction int

const (
	menuStay   menuAction = iota // redisplay the menu (with a message)
	menuOpen                     // open the menu item with that key
	menuLogoff                   // end the logon, back to the logon screen
	menuExit                     // disconnect
)

// menuCommand decides what the menu does with a key and command (#45).
// F3 and F12 go back everywhere else, so here they do nothing: leaving Jane
// takes a deliberate L (log off, only when logons are required) or X (exit).
func menuCommand(aid go3270.AID, cmd string, logons bool) (menuAction, string) {
	if isBack(aid) {
		if logons {
			return menuStay, "This is the top menu: type L to log off or X to exit"
		}
		return menuStay, "This is the top menu: type X to exit"
	}
	if aid != go3270.AIDEnter {
		return menuStay, ""
	}
	switch strings.ToUpper(strings.TrimSpace(cmd)) {
	case "":
		return menuStay, ""
	case "X", "=X", "EXIT":
		return menuExit, ""
	case "L", "=L", "LOGOFF", "LOGOUT":
		if logons {
			return menuLogoff, ""
		}
	}
	return menuOpen, ""
}

// menuTx is the primary option menu. L logs off (when logons are required),
// X exits; F3 and F12 do not leave it.
func menuTx(s store) go3270.Tx {
	var self go3270.Tx
	message := ""
	self = func(conn net.Conn, dev go3270.DevInfo, _ any) (go3270.Tx, any, error) {
		if !loggedOn(conn) {
			return logonTx(s), nil, nil
		}
		sess := sessionOf(conn)
		logons := sess != nil && sess.auth.Enabled()

		keys := "X=Exit"
		if logons {
			keys = "L=Logoff  X=Exit"
		}
		screen := frame("Primary Option Menu", "", message, "Enter=Select  "+keys)
		row := 4
		for _, m := range menuItems {
			screen = append(screen, go3270.Field{Row: row, Col: 2, Intense: true, Content: m.Key},
				go3270.Field{Row: row, Col: 5, Content: cut(m.Label, 74)})
			row++
		}
		row++
		if logons {
			screen = append(screen, go3270.Field{Row: row, Col: 2, Intense: true, Content: "L"},
				go3270.Field{Row: row, Col: 5, Content: "Logoff            end this logon and return to the logon screen"})
			row++
		}
		screen = append(screen, go3270.Field{Row: row, Col: 2, Intense: true, Content: "X"},
			go3270.Field{Row: row, Col: 5, Content: "Exit              disconnect from Jane"},
			go3270.Field{Row: 18, Col: 2, Color: go3270.Blue, Content: "Type an option on the command line and press Enter. F3 goes back to here."},
			go3270.Field{Row: 19, Col: 2, Color: go3270.Blue, Content: "In lists, type S beside a row (or its number on the command line) to open it."})
		message = ""

		resp, err := show(conn, dev, screen)
		if err != nil {
			return nil, nil, err
		}
		action, msg := menuCommand(resp.AID, resp.Values["cmd"], logons)
		cmd := strings.ToUpper(strings.TrimSpace(resp.Values["cmd"]))
		switch action {
		case menuStay:
			message = msg
			return self, nil, nil
		case menuExit:
			if sess != nil {
				sess.ending = "exit"
			}
			return nil, nil, nil
		case menuLogoff:
			return logoff(conn, s), nil, nil
		}
		for _, m := range menuItems {
			if cmd == m.Key {
				return m.Open(s, self), nil, nil
			}
		}
		message = fmt.Sprintf("Option %q is not valid", clean(cmd))
		return self, nil, nil
	}
	return self
}

// ---------------------------------------------------------------- lists

// listItem is one row of a list and how to open it.
type listItem struct {
	Text   string        // the row text, after the row number
	Color  go3270.Color  // optional; colours the row (always alongside its text)
	Detail func() detail // builds the detail screen when selected
	// Open, if set, is used instead of Detail: it returns the screen to go
	// to when the row is selected (back is the list itself).
	Open func(back go3270.Tx) go3270.Tx
}

// open returns the screen for a selected row.
func (it listItem) open(back go3270.Tx) go3270.Tx {
	if it.Open != nil {
		return it.Open(back)
	}
	return textTx(it.Detail(), back)
}

// detail is the content of a detail screen.
type detail struct {
	Title   string
	Lines   []string
	Err     error
	Message string // shown on the message line when there is no Err
	// Links makes lines selectable: put the cursor on the line and press
	// Enter to open what it refers to. Keyed by line index.
	Links map[int]link
	// Action, if set, is one extra key for this screen (e.g. F9 to close a
	// session, #98). Do returns the screen to show afterwards.
	Action *detailAction
}

// detailAction is a key offered on one detail screen.
type detailAction struct {
	Key   go3270.AID
	Label string // e.g. "F9=Close session", added to the key line
	Do    func(conn net.Conn) detail
}

// link is what a selectable line opens. Spans narrow it: a span covering the
// character under the cursor wins over the line's own target.
type link struct {
	Open  func() detail
	Spans []span
}

// span is a range of character positions [From, To) within a line.
type span struct {
	From, To int
	Open     func() detail
}

// at returns what to open for the character at position ch, if anything.
func (l link) at(ch int) func() detail {
	for _, sp := range l.Spans {
		if ch >= sp.From && ch < sp.To {
			return sp.Open
		}
	}
	return l.Open
}

// add appends lines; if l is given it applies to each of them.
func (d *detail) add(lines []string, l ...link) {
	for _, line := range lines {
		if len(l) > 0 {
			if d.Links == nil {
				d.Links = map[int]link{}
			}
			d.Links[len(d.Lines)] = l[0]
		}
		d.Lines = append(d.Lines, line)
	}
}

// prepend puts lines before the existing ones, keeping links in step.
func (d *detail) prepend(lines []string, links map[int]link) {
	shifted := map[int]link{}
	for i, l := range links {
		shifted[i] = l
	}
	for i, l := range d.Links {
		shifted[i+len(lines)] = l
	}
	d.Lines = append(append([]string{}, lines...), d.Lines...)
	if len(shifted) > 0 {
		d.Links = shifted
	}
}

// listSource feeds a list screen a page at a time.
type listSource struct {
	Title  string
	Header string
	// Fetch returns up to limit items starting at skip, and the total count.
	Fetch func(skip, limit int64) ([]listItem, int64, error)
}

// listTx shows a source a page at a time. Type S beside a row, or the row's
// number on the command line, to open it; F7/F8 page; F5 refreshes.
func listTx(src listSource, back go3270.Tx) go3270.Tx {
	var self go3270.Tx
	top := int64(0) // index of the first row on the page
	message := ""
	self = func(conn net.Conn, dev go3270.DevInfo, _ any) (go3270.Tx, any, error) {
		items, total, ferr := src.Fetch(top, listRows)
		if ferr == nil && top > 0 && top >= total {
			// the list shrank since we last looked: go back to its last page
			top = max(0, (total-1)/listRows*listRows)
			items, total, ferr = src.Fetch(top, listRows)
		}
		if ferr != nil {
			message = "Could not read the list: " + ferr.Error()
		}

		pos := "No entries"
		if total > 0 {
			pos = fmt.Sprintf("Row %d to %d of %d", top+1, top+int64(len(items)), total)
		}
		screen := frame(src.Title, pos, message, "F3=Back  F5=Refresh  F7=Up  F8=Down  S=Select  F12=Cancel")
		screen = append(screen, go3270.Field{Row: headRow, Col: rowCol, Intense: true, Content: fit("  # "+src.Header, rowWidth)})
		for i, it := range items {
			r := firstRow + i
			screen = append(screen,
				go3270.Field{Row: r, Col: selCol, Write: true, Name: fmt.Sprintf("sel%d", i), Highlighting: go3270.Underscore},
				go3270.Field{Row: r, Col: rowCol, Color: it.Color, Content: fit(fmt.Sprintf("%3d %s", top+int64(i)+1, it.Text), rowWidth)})
		}
		if len(items) == 0 && ferr == nil {
			screen = append(screen, go3270.Field{Row: firstRow, Col: rowCol, Content: "No entries."})
		}
		message = ""

		resp, err := show(conn, dev, screen)
		if err != nil {
			return nil, nil, err
		}
		switch resp.AID {
		case go3270.AIDPF3, go3270.AIDPF12:
			return back, nil, nil
		case go3270.AIDPF7:
			top = max(0, top-listRows)
			return self, nil, nil
		case go3270.AIDPF8:
			if top+listRows < total {
				top += listRows
			} else {
				message = "Already at the end of the list"
			}
			return self, nil, nil
		case go3270.AIDPF5:
			return self, nil, nil
		case go3270.AIDEnter:
		default:
			return self, nil, nil
		}

		// a row marked S (or any character) beside it
		for i := range items {
			if v := strings.TrimSpace(resp.Values[fmt.Sprintf("sel%d", i)]); v != "" {
				return items[i].open(self), nil, nil
			}
		}
		// or a row number on the command line
		cmd := strings.TrimSpace(resp.Values["cmd"])
		if cmd == "" {
			return self, nil, nil
		}
		n, perr := strconv.ParseInt(cmd, 10, 64)
		if perr != nil {
			message = fmt.Sprintf("Type a row number or S beside a row, not %q", clean(cmd))
			return self, nil, nil
		}
		if n < 1 || n > total {
			message = fmt.Sprintf("Row %d is not in the list", n)
			return self, nil, nil
		}
		if n-1 < top || n-1 >= top+int64(len(items)) {
			top = (n - 1) / listRows * listRows // jump to the page holding row n
			got, _, gerr := src.Fetch(top, listRows)
			if gerr != nil || int(n-1-top) >= len(got) {
				message = fmt.Sprintf("Row %d is not in the list", n)
				return self, nil, nil
			}
			return got[n-1-top].open(self), nil, nil
		}
		return items[n-1-top].open(self), nil, nil
	}
	return self
}

// ---------------------------------------------------------------- text

// textTx shows the lines of a detail screen; F7/F8 scroll, F3 goes back.
func textTx(d detail, back go3270.Tx) go3270.Tx {
	var self go3270.Tx
	top := 0
	message := d.Message
	if d.Err != nil {
		message = d.Err.Error()
	}
	// where to put the cursor: the command line, or the line last opened
	curRow, curCol := cmdRow, cmdCol+1
	keys := "F3=Back  F7=Up  F8=Down  F12=Cancel"
	if len(d.Links) > 0 {
		keys = "F3=Back  F7=Up  F8=Down  Enter=Open the white line under the cursor"
	}
	if d.Action != nil {
		keys = "F3=Back  F7=Up  F8=Down  " + d.Action.Label
	}
	self = func(conn net.Conn, dev go3270.DevInfo, _ any) (go3270.Tx, any, error) {
		end := min(top+textRows, len(d.Lines))
		pos := "Empty"
		if len(d.Lines) > 0 {
			pos = fmt.Sprintf("Line %d to %d of %d", top+1, end, len(d.Lines))
		}
		screen := frame(d.Title, pos, message, keys)
		for i, l := range d.Lines[top:end] {
			f := go3270.Field{Row: headRow + i, Col: 0, Content: cut(l, lineWidth)}
			if _, ok := d.Links[top+i]; ok {
				f.Color = go3270.White
			}
			screen = append(screen, f)
		}
		message = ""

		resp, err := send(conn, dev, screen, go3270.ScreenOpts{
			CursorRow: curRow, CursorCol: curCol, Codepage: dev.Codepage(),
		})
		curRow, curCol = cmdRow, cmdCol+1
		if err != nil {
			return nil, nil, err
		}
		if d.Action != nil && resp.AID == d.Action.Key {
			return textTx(d.Action.Do(conn), back), nil, nil
		}
		switch resp.AID {
		case go3270.AIDPF3, go3270.AIDPF12:
			return back, nil, nil
		case go3270.AIDPF7:
			top = max(0, top-textRows)
			return self, nil, nil
		case go3270.AIDPF8:
			if top+textRows < len(d.Lines) {
				top += textRows
			} else {
				message = "Already at the end"
			}
			return self, nil, nil
		case go3270.AIDEnter:
		default:
			return self, nil, nil
		}

		cmd := strings.ToUpper(strings.TrimSpace(resp.Values["cmd"]))
		switch cmd {
		case "TOP", "T":
			top = 0
			return self, nil, nil
		case "BOTTOM", "BOT", "B":
			top = max(0, (len(d.Lines)-1)/textRows*textRows)
			return self, nil, nil
		case "":
		default:
			message = fmt.Sprintf("Unknown command %q: use TOP or BOT, or F3 to go back", clean(cmd))
			return self, nil, nil
		}

		// Enter with the cursor on a selectable line opens it. A field's
		// content starts one column after its attribute byte at column 0.
		if row := resp.Row; row >= headRow && row < headRow+(end-top) {
			if l, ok := d.Links[top+row-headRow]; ok {
				if open := l.at(resp.Col - 1); open != nil {
					curRow, curCol = resp.Row, resp.Col
					return textTx(open(), self), nil, nil
				}
			}
		}
		return self, nil, nil
	}
	return self
}
