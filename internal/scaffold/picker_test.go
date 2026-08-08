package scaffold

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func pickerOptions() []pickerItem {
	return []pickerItem{
		{label: "New", value: "New"},
		{label: "opencode — OpenCode agent", value: "opencode"},
		{label: "mise — Shared toolchain base", value: "mise"},
	}
}

func TestPickerEnterSelects(t *testing.T) {
	m := newPickerModel("Select a built-in profile", pickerOptions(), nil)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = u.(pickerModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(pickerModel)
	if !m.done || m.result != "opencode" {
		t.Errorf("enter should select the highlighted option, done=%v result=%q", m.done, m.result)
	}
}

func TestPickerEscCancels(t *testing.T) {
	m := newPickerModel("Select a built-in profile", pickerOptions(), nil)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(pickerModel)
	if !m.cancelled || !m.done {
		t.Error("esc should cancel the picker")
	}
}

func TestPickerDetailsOpensAndCloses(t *testing.T) {
	contents := map[string]string{"opencode": "version: 1\nimage: debian:13-slim\ncommand: [opencode]\n"}
	m := newPickerModel("Select a built-in profile", pickerOptions(), contents)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown}) // opencode
	m = u.(pickerModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = u.(pickerModel)
	if !m.inspecting {
		t.Fatal("d should open the details popup")
	}
	v := stripANSI(m.View())
	for _, want := range []string{"opencode", "version: 1", "image: debian:13-slim", "command: [opencode]"} {
		if !strings.Contains(v, want) {
			t.Errorf("details view missing %q\n%s", want, v)
		}
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(pickerModel)
	if m.inspecting {
		t.Error("esc should close the details popup")
	}
}

func TestPickerDetailsWithoutContentsDoesNothing(t *testing.T) {
	m := newPickerModel("Select a built-in profile", pickerOptions(), nil)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}) // "New" has no contents
	m = u.(pickerModel)
	if m.inspecting {
		t.Error("d on an entry without contents must not open a popup")
	}
	if m.done {
		t.Error("d must not select the entry")
	}
}

func TestPickerDetailsScrolls(t *testing.T) {
	var b strings.Builder
	b.WriteString("version: 1\n")
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&b, "line: %d\n", i)
	}
	m := newPickerModel("Select a built-in profile", []pickerItem{
		{label: "New", value: "New"},
		{label: "big", value: "big"},
	}, map[string]string{"big": b.String()})
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = u.(pickerModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = u.(pickerModel)
	if v := stripANSI(m.View()); !strings.Contains(v, "↑/↓ scroll") {
		t.Fatalf("overflowing content should show a scroll hint\n%s", v)
	}
	for i := 0; i < 20; i++ {
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = u.(pickerModel)
	}
	if m.detailScroll == 0 {
		t.Fatal("down should advance the scroll offset")
	}
	max := m.detailScroll
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = u.(pickerModel)
	if m.detailScroll != max {
		t.Errorf("down past the end should clamp, got %d want %d", m.detailScroll, max)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "line: 19") || !strings.Contains(v, "line: 20") {
		t.Errorf("bottom of the content should be reachable\n%s", v)
	}
	for i := 0; i < 20; i++ {
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = u.(pickerModel)
	}
	if m.detailScroll != 0 {
		t.Errorf("up past the top should clamp to 0, got %d", m.detailScroll)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "line: 1") || !strings.Contains(v, "version: 1") {
		t.Errorf("top of the content should be reachable\n%s", v)
	}
}

func TestPickerDetailsGrowsShortListAndRestores(t *testing.T) {
	// A two-option list yields a frame shorter than the popup; the viewport
	// must grow so the popup content is fully visible, then shrink back.
	contents := map[string]string{"mise": "version: 1\nimage: debian:13-slim\ncommand: [\"/usr/bin/mise\"]\npackages:\n  - mise\ncaches:\n  mise: ~/.local/share/mise\n"}
	m := newPickerModel("Select a built-in profile", []pickerItem{
		{label: "New", value: "New"},
		{label: "mise", value: "mise"},
	}, contents)
	naturalH := m.naturalH
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = u.(pickerModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = u.(pickerModel)
	if m.list.Height() <= naturalH {
		t.Errorf("details should grow the short list, height=%d natural=%d", m.list.Height(), naturalH)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "image: debian:13-slim") {
		t.Errorf("popup content must be fully visible\n%s", v)
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(pickerModel)
	if m.list.Height() != naturalH {
		t.Errorf("closing details should restore the list height, height=%d natural=%d", m.list.Height(), naturalH)
	}
}

func TestNewProfileEnterAdvancesToBaseList(t *testing.T) {
	m := newNewProfileModel(pickerOptions(), nil)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = u.(newProfileModel)
	if m.name.Value() != "x" {
		t.Errorf("typing should go to the name input, got %q", m.name.Value())
	}
	if m.focused {
		t.Error("the base list must not be focused while typing a name")
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(newProfileModel)
	if !m.focused {
		t.Error("enter on the name input should focus the base list")
	}
	if m.done {
		t.Error("enter on the name input must not submit")
	}
}

func TestNewProfileSubmitReturnsNameAndBase(t *testing.T) {
	m := newNewProfileModel(pickerOptions(), nil)
	for _, r := range "myagent" {
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = u.(newProfileModel)
	}
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // advance to the base list
	m = u.(newProfileModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // opencode
	m = u.(newProfileModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // submit
	m = u.(newProfileModel)
	if !m.done || m.nameResult != "myagent" || m.baseResult != "opencode" {
		t.Errorf("submit should return name and base, done=%v name=%q base=%q", m.done, m.nameResult, m.baseResult)
	}
}

func TestNewProfileDetailsOpensAndCloses(t *testing.T) {
	contents := map[string]string{"opencode": "version: 1\nimage: debian:13-slim\n"}
	m := newNewProfileModel(pickerOptions(), contents)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // advance to the base list
	m = u.(newProfileModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // opencode
	m = u.(newProfileModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = u.(newProfileModel)
	if !m.inspecting {
		t.Fatal("d should open the details popup on a base")
	}
	v := stripANSI(m.View())
	for _, want := range []string{"opencode", "version: 1", "image: debian:13-slim"} {
		if !strings.Contains(v, want) {
			t.Errorf("details view missing %q\n%s", want, v)
		}
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(newProfileModel)
	if m.inspecting {
		t.Error("esc should close the details popup")
	}
}

func TestNewProfileEscCancels(t *testing.T) {
	for _, key := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyCtrlC}} {
		m := newNewProfileModel(pickerOptions(), nil)
		u, _ := m.Update(key)
		m = u.(newProfileModel)
		if !m.cancelled || !m.done {
			t.Errorf("%s should cancel the new-profile screen", key.String())
		}
	}
}

func TestNewProfileTabSwitchesBackToName(t *testing.T) {
	m := newNewProfileModel(pickerOptions(), nil)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // advance to the base list
	m = u.(newProfileModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = u.(newProfileModel)
	if m.focused {
		t.Error("tab on the base list should return focus to the name input")
	}
}

func TestNewProfileDetailsGrowsShortListAndRestores(t *testing.T) {
	contents := map[string]string{"mise": "version: 1\nimage: debian:13-slim\ncommand: [\"/usr/bin/mise\"]\npackages:\n  - mise\ncaches:\n  mise: ~/.local/share/mise\n"}
	m := newNewProfileModel([]pickerItem{
		{label: "mise", value: "mise"},
		{label: "opencode", value: "opencode"},
	}, contents)
	naturalH := m.naturalH
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // advance to the base list
	m = u.(newProfileModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = u.(newProfileModel)
	if m.list.Height() <= naturalH {
		t.Errorf("details should grow the short base list, height=%d natural=%d", m.list.Height(), naturalH)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "image: debian:13-slim") {
		t.Errorf("popup content must be fully visible\n%s", v)
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(newProfileModel)
	if m.list.Height() != naturalH {
		t.Errorf("closing details should restore the base list height, height=%d natural=%d", m.list.Height(), naturalH)
	}
}
