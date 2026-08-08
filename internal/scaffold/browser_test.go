package scaffold

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBuildFragmentNav(t *testing.T) {
	names := []string{"cloud/aws", "cloud/azure", "gui/display", "toolchain/go", "toolchain/javascript", "top"}
	descs := map[string]string{"toolchain/go": "Go toolchain with GOPATH cache", "top": "top-level"}
	nav := buildFragmentNav(names, descs)
	if !reflect.DeepEqual(nav.folders, []string{"cloud", "gui", "toolchain"}) {
		t.Errorf("folders = %v, want [cloud gui toolchain]", nav.folders)
	}
	cloud := nav.byFolder["cloud"]
	if len(cloud) != 2 || cloud[0].display != "cloud/aws" || cloud[1].display != "cloud/azure" {
		t.Errorf("cloud items = %v", cloud)
	}
	if got := nav.byFolder["toolchain"][0]; got.label != "go — Go toolchain with GOPATH cache" || got.kind != itemFragment {
		t.Errorf("toolchain/go row = %+v", got)
	}
	if len(nav.topFrags) != 1 || nav.topFrags[0].display != "top" || nav.topFrags[0].label != "top — top-level" {
		t.Errorf("top frags = %+v", nav.topFrags)
	}
}

func TestBuildFragmentNavFlattensDeep(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/js/node", "toolchain/js/tsc"}, nil)
	if !reflect.DeepEqual(nav.folders, []string{"toolchain"}) {
		t.Errorf("folders = %v, want [lang]", nav.folders)
	}
	items := nav.byFolder["toolchain"]
	if len(items) != 2 || items[0].display != "toolchain/js/node" || items[0].label != "js/node" {
		t.Errorf("deep fragments flatten to their remainder: %+v", items)
	}
}

func TestBuildFragmentNavEmpty(t *testing.T) {
	nav := buildFragmentNav(nil, nil)
	if len(nav.folders) != 0 || len(nav.topFrags) != 0 || len(nav.byFolder) != 0 {
		t.Errorf("empty nav = %+v", nav)
	}
}

func TestFragmentNavItemLabels(t *testing.T) {
	descs := map[string]string{"toolchain/go": "Go toolchain"}
	if got := fragmentRow("toolchain/go", "go", descs).label; got != "go — Go toolchain" {
		t.Errorf("described label = %q", got)
	}
	if got := fragmentRow("vcs/git", "git", descs).label; got != "git" {
		t.Errorf("undescribed label = %q", got)
	}
}

func TestFolderNavEnterExpandsFolder(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go", "toolchain/javascript", "vcs/git"}, nil)
	m := newFolderNavModel("Folder", nav, nil)
	if len(m.list.VisibleItems()) != 2 {
		t.Fatalf("initial rows = %d, want 2", len(m.list.VisibleItems()))
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(folderNavModel)
	if !m.expanded["toolchain"] {
		t.Error("enter should expand the highlighted folder")
	}
	if len(m.list.VisibleItems()) != 4 {
		t.Errorf("expanded rows = %d, want 4", len(m.list.VisibleItems()))
	}
	if cmd != nil {
		t.Error("expanding must not quit the program")
	}
	if it := m.list.SelectedItem().(folderNavItem); it.display != "toolchain" {
		t.Errorf("cursor moved off the folder to %q", it.display)
	}
}

func TestFolderNavEnterCollapsesFolder(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go", "toolchain/javascript"}, nil)
	m := newFolderNavModel("Folder", nav, nil)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(folderNavModel)
	if m.expanded["toolchain"] {
		t.Error("second enter should collapse the folder")
	}
	if len(m.list.VisibleItems()) != 1 {
		t.Errorf("collapsed rows = %d, want 1", len(m.list.VisibleItems()))
	}
}

func TestFolderNavSpaceTogglesFragment(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go", "toolchain/javascript"}, nil)
	m := newFolderNavModel("Folder", nav, nil)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = u.(folderNavModel)
	if !m.picked["toolchain/go"] {
		t.Error("space should pick the highlighted fragment")
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = u.(folderNavModel)
	if m.picked["toolchain/go"] {
		t.Error("space should unpick a picked fragment")
	}
}

func TestFolderNavSpaceExpandsFolder(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go"}, nil)
	m := newFolderNavModel("Folder", nav, nil)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(folderNavModel)
	if !m.expanded["toolchain"] {
		t.Error("space should expand the highlighted folder")
	}
	if len(m.list.VisibleItems()) != 2 {
		t.Errorf("expanded rows = %d, want 2", len(m.list.VisibleItems()))
	}
	if cmd != nil {
		t.Error("expanding must not quit the program")
	}
}

func TestFolderNavEnterTogglesFragment(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go"}, nil)
	m := newFolderNavModel("Folder", nav, nil)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = u.(folderNavModel)
	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(folderNavModel)
	if !m.picked["toolchain/go"] {
		t.Error("enter should pick the highlighted fragment")
	}
	if cmd != nil {
		t.Error("toggling must not quit the program")
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(folderNavModel)
	if m.picked["toolchain/go"] {
		t.Error("enter should unpick a picked fragment")
	}
}

func TestFolderNavTabFocusesDone(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go"}, nil)
	m := newFolderNavModel("Folder", nav, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(folderNavModel)
	if !m.focused {
		t.Fatal("tab should focus the Done section")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if res := updated.(folderNavModel).result; res != browserDone {
		t.Errorf("enter on focused Done = %q, want %s", res, browserDone)
	}
}

func TestFolderNavTabTogglesBackToList(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go"}, nil)
	m := newFolderNavModel("Folder", nav, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(folderNavModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if updated.(folderNavModel).focused {
		t.Error("second tab should return focus to the list")
	}
}

func TestFolderNavEscCancels(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go"}, nil)
	m := newFolderNavModel("Folder", nav, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if res := updated.(folderNavModel).result; res != browserCancel {
		t.Errorf("esc = %q, want %s", res, browserCancel)
	}
}

func TestFolderNavCtrlCCancels(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go"}, nil)
	m := newFolderNavModel("Folder", nav, nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if res := updated.(folderNavModel).result; res != browserCancel {
		t.Errorf("ctrl+c = %q, want %s", res, browserCancel)
	}
}

func TestFolderNavDetailsOpensAndCloses(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go", "toolchain/javascript"}, nil)
	contents := map[string]string{"toolchain/go": "caches:\n  go: ~/go\ntools:\n  go: latest\n"}
	m := newFolderNavModel("Folder", nav, contents)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand toolchain
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // highlight toolchain/go
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = u.(folderNavModel)
	if !m.inspecting {
		t.Fatal("d should open the details popup on a fragment")
	}
	v := stripANSI(m.View())
	for _, want := range []string{"toolchain/go", "caches:", "go: ~/go", "tools:", "go: latest"} {
		if !strings.Contains(v, want) {
			t.Errorf("details view missing %q\n%s", want, v)
		}
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = u.(folderNavModel)
	if m.inspecting {
		t.Error("esc should close the details popup")
	}
	if !strings.Contains(stripANSI(m.View()), "▾ toolchain") {
		t.Error("list should render again after closing details")
	}
}

func TestFolderNavDetailsIgnoresFolders(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go"}, nil)
	m := newFolderNavModel("Folder", nav, map[string]string{"toolchain/go": "caches:\n  go: ~/go\n"})
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}) // cursor on the folder
	m = u.(folderNavModel)
	if m.inspecting {
		t.Error("d on a folder must not open a details popup")
	}
}

func TestFolderNavDetailsCtrlCCancels(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go"}, nil)
	m := newFolderNavModel("Folder", nav, map[string]string{"toolchain/go": "caches:\n  go: ~/go\n"})
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand toolchain
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // highlight the fragment
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = u.(folderNavModel)
	if !m.inspecting {
		t.Fatal("d should open the details popup")
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = u.(folderNavModel)
	if m.result != browserCancel || !m.done {
		t.Error("ctrl+c from the details popup should cancel the browser")
	}
}

func TestFolderNavDetailsScrolls(t *testing.T) {
	var b strings.Builder
	b.WriteString("version: 1\n")
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&b, "line: %d\n", i)
	}
	nav := buildFragmentNav([]string{"toolchain/go"}, nil)
	m := newFolderNavModel("Folder", nav, map[string]string{"toolchain/go": b.String()})
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand toolchain
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // highlight the fragment
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = u.(folderNavModel)
	if v := stripANSI(m.View()); !strings.Contains(v, "↑/↓ scroll") {
		t.Fatalf("overflowing fragment should show a scroll hint\n%s", v)
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = u.(folderNavModel)
	if m.detailScroll == 0 {
		t.Error("down should scroll the details")
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = u.(folderNavModel)
	if m.detailScroll != 0 {
		t.Error("up should scroll back toward the top")
	}
}

func TestRootFragmentsAlignWithFolders(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go", "defaults"}, nil)
	m := newFolderNavModel("Folder", nav, nil)
	rowCol := func(sub string) int {
		for _, l := range strings.Split(stripANSI(m.View()), "\n") {
			if i := strings.Index(l, sub); i >= 0 {
				return i
			}
		}
		return -1
	}
	folderCol := rowCol("▸ toolchain")
	fragCol := rowCol("• defaults")
	if folderCol < 0 || fragCol < 0 {
		t.Fatalf("missing rows: folder=%d fragment=%d\n%s", folderCol, fragCol, m.View())
	}
	if fragCol != folderCol {
		t.Errorf("root fragment should align with folders: folder at %d, fragment at %d", folderCol, fragCol)
	}
}

func TestFolderNavDelegateMarkers(t *testing.T) {
	nav := buildFragmentNav([]string{"toolchain/go", "toolchain/javascript", "vcs/git"}, nil)
	m := newFolderNavModel("Folder", nav, nil)
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand toolchain
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // highlight toolchain/go
	m = u.(folderNavModel)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace}) // pick toolchain/go
	m = u.(folderNavModel)
	v := m.View()
	for _, want := range []string{"▾ toolchain", "✓ go", "• javascript"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q in view:\n%s", want, v)
		}
	}
	if strings.Contains(v, "▸ toolchain") {
		t.Errorf("expanded folder must show ▾, got:\n%s", v)
	}
}
