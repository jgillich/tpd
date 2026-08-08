package scaffold

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jgillich/tpd/internal/ui"
)

// Styles mirror huh's ThemeCharm so the folder nav looks identical to the
// huh prompts around it. Colors come straight from huh/theme.go.
var (
	fuchsia  = lipgloss.Color("#F780E2")
	cream    = lipgloss.Color("#FFFDF5")
	indigo   = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	normalFg = lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
	green    = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
	buttonBG = lipgloss.AdaptiveColor{Light: "252", Dark: "237"}

	// One FocusedButton/BlurredButton style each, like huh's Confirm field.
	doneButtonFocused = lipgloss.NewStyle().
				Padding(0, 2).
				MarginRight(1).
				Foreground(cream).
				Background(fuchsia)
	doneButtonBlurred = lipgloss.NewStyle().
				Padding(0, 2).
				MarginRight(1).
				Foreground(normalFg).
				Background(buttonBG)
	// Field title: indigo bold — not bubbles/list's default filled box.
	folderTitle  = lipgloss.NewStyle().Foreground(indigo).Bold(true)
	folderCursor = lipgloss.NewStyle().Foreground(fuchsia)
	// Folder rows use huh's select option colors (green highlighted, normal
	// otherwise); fragment rows reuse the pair for picked/unpicked text.
	folderLabel    = lipgloss.NewStyle().Foreground(green)
	folderLabelDim = lipgloss.NewStyle().Foreground(normalFg)
	// huh MultiSelect pick prefixes.
	fragPickedPrefix   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#02CF92", Dark: "#02A877"})
	fragUnpickedPrefix = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "", Dark: "243"})
	// bubbles/help defaults are exactly what huh's ThemeCharm help uses.
	navHelp         = help.New()
	clearBelowFrame = "\x1b[0J" // erase from cursor to end of screen
	// navPopupStyle is the details popup: a bordered box with no background, so
	// the list around it stays visible while the popup's own cells overwrite
	// the covered rows. Mirrors the approval prompt's popup.
	navPopupStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("254")).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(fuchsia)
	// Scrollbar inside the details popup: a fuchsia thumb on a faint track,
	// like the approval list's scrollbar.
	navThumb = lipgloss.NewStyle().Foreground(fuchsia)
	navTrack = lipgloss.NewStyle().Faint(true)
)

// sectionStyle returns huh's field base: a left thick border plus 1-space pad
// around the content. The blurred variant swaps the visible border for a
// hidden one, so the content never shifts when focus moves between sections.
func sectionStyle(focused bool) lipgloss.Style {
	s := lipgloss.NewStyle().
		PaddingLeft(1).
		BorderStyle(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderForeground(lipgloss.Color("238"))
	if !focused {
		s = s.BorderStyle(lipgloss.HiddenBorder())
	}
	return s
}

// folderNavKeyBinds are the help bindings for each section, in huh's
// "↑ up • ↓ down • …" short-help format.
var (
	listKeyBinds = []key.Binding{
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑", "up")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓", "down")),
		key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("enter/space", "toggle")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "details")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "done")),
		key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "cancel")),
	}
	doneKeyBinds = []key.Binding{
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "list")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit")),
		key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "cancel")),
	}
)

type navItemKind int

const (
	itemFolder navItemKind = iota
	itemFragment
)

// folderNavItem is one row of the folder-navigation list: a folder that
// expands on Enter to reveal its fragments, or a fragment toggled with Space.
type folderNavItem struct {
	label   string // folder name, or "leaf — desc" for a fragment
	display string // folder name, or the fragment's full display name
	kind    navItemKind
}

func (i folderNavItem) FilterValue() string {
	if i.kind == itemFragment {
		return i.display + " " + i.label
	}
	return i.label
}

// folderNavDelegate renders rows like huh: tight single lines with a fuchsia
// cursor on the highlighted row. Folders show a ▸/▾ expand marker; fragments
// indent below their folder and carry the MultiSelect ✓/• pick markers.
type folderNavDelegate struct {
	list.DefaultDelegate
	expanded map[string]bool
	picked   map[string]bool
}

func (d folderNavDelegate) Height() int  { return 1 }
func (d folderNavDelegate) Spacing() int { return 0 }

func (d folderNavDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it := item.(folderNavItem)
	cursor := "  "
	if index == m.Index() {
		cursor = folderCursor.Render(">") + " "
	}
	switch it.kind {
	case itemFolder:
		marker := "▸ "
		if d.expanded[it.display] {
			marker = "▾ "
		}
		label := folderLabelDim
		if index == m.Index() {
			label = folderLabel
		}
		fmt.Fprintf(w, "%s%s%s", cursor, marker, label.Render(it.label))
	case itemFragment:
		prefix := fragUnpickedPrefix.Render("• ")
		label := folderLabelDim
		if d.picked[it.display] {
			prefix = fragPickedPrefix.Render("✓ ")
			label = folderLabel
		}
		if strings.Contains(it.display, "/") {
			// Nested fragments indent one level under their folder.
			fmt.Fprintf(w, "%s  %s%s", cursor, prefix, label.Render(it.label))
		} else {
			// Root-level fragments (no "/") align with the folder rows.
			fmt.Fprintf(w, "%s%s%s", cursor, prefix, label.Render(it.label))
		}
	}
}

// folderNavModel is the bubbletea model for the folder-navigation screen. Like
// huh, it has two always-visible sections — the folder list and a Done button
// — switched with tab; the list scrolls internally while the button stays put.
type folderNavModel struct {
	list        list.Model
	nav         fragmentNav
	contents    map[string]string // fragment display name → YAML contents for the details popup
	availHeight int               // list viewport rows once a WindowSizeMsg arrives (0 before)
	width       int               // content width, min 0 until a WindowSizeMsg arrives
	naturalH    int               // list viewport height with no details popup open
	focused     bool              // true when the Done button section is focused
	done        bool              // true once an action has been chosen (frame clears on exit)
	expanded    map[string]bool
	picked      map[string]bool
	inspecting  bool // true while the details popup is open
	detailScroll int // scroll offset into the details popup's content
	result      string
}

func newFolderNavModel(title string, nav fragmentNav, contents map[string]string) folderNavModel {
	expanded := map[string]bool{}
	picked := map[string]bool{}
	d := folderNavDelegate{expanded: expanded, picked: picked}
	d.ShowDescription = false
	items := visibleItems(nav, expanded)
	// Content-sized list: title plus one tight row per item. The Done button, a
	// blank separator, and the help render below, outside the list viewport, so
	// they stay visible even when the folder list scrolls.
	l := list.New(toListItems(items), d, 80, len(items)+1)
	l.Title = title
	l.Styles.Title = folderTitle
	l.Styles.TitleBar = lipgloss.NewStyle()
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.SetShowTitle(true)
	// Quit is handled by this model so cancelling can report back to the
	// browser loop; esc/q clear an applied filter before quitting.
	l.KeyMap.Quit = key.NewBinding(key.WithDisabled())
	return folderNavModel{list: l, nav: nav, contents: contents, expanded: expanded, picked: picked, naturalH: len(items) + 1}
}

// visibleItems returns the rows in display order: each folder followed by its
// fragments when expanded, then the root-level fragments.
func visibleItems(nav fragmentNav, expanded map[string]bool) []folderNavItem {
	var items []folderNavItem
	for _, f := range nav.folders {
		items = append(items, folderNavItem{kind: itemFolder, label: f, display: f})
		if expanded[f] {
			items = append(items, nav.byFolder[f]...)
		}
	}
	return append(items, nav.topFrags...)
}

func toListItems(items []folderNavItem) []list.Item {
	out := make([]list.Item, len(items))
	for i, it := range items {
		out[i] = it
	}
	return out
}

// rebuild refreshes the list after an expand/collapse. SetItems keeps the
// cursor on the toggled folder row; the viewport grows up to the window height
// so an expanded folder's fragments show without scrolling.
func (m folderNavModel) rebuild() (folderNavModel, tea.Cmd) {
	cmd := m.list.SetItems(toListItems(visibleItems(m.nav, m.expanded)))
	m.applyListHeight()
	return m, cmd
}

// applyListHeight sizes the list to its natural height (the visible items,
// capped by the window) and records it, so the details popup can grow the
// viewport and later restore it.
func (m *folderNavModel) applyListHeight() {
	m.naturalH = m.listHeight()
	m.list.SetHeight(m.naturalH)
}

// popupListHeight is the list viewport height that makes the frame tall enough
// to fully render the details popup. The frame is list + blank + Done button +
// blank + help, so it needs the popup's height minus that 4-row overhead.
func (m folderNavModel) popupListHeight() int {
	need := len(strings.Split(m.popupBox(), "\n")) - 4
	if need < 1 {
		need = 1
	}
	if need < m.naturalH {
		return m.naturalH
	}
	return need
}

func (m folderNavModel) listHeight() int {
	h := len(visibleItems(m.nav, m.expanded)) + 1
	if m.availHeight > 0 && h > m.availHeight {
		h = m.availHeight
	}
	if h < 1 {
		h = 1
	}
	return h
}

func (m folderNavModel) Init() tea.Cmd { return nil }

func (m folderNavModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.String() == "ctrl+c":
			// Take precedence over the list's ForceQuit: without this the
			// program would quit with an empty result, which the browser loop
			// misreads as "finalize" and continues the wizard.
			m.result = browserCancel
			m.done = true
			return m, tea.Quit
		case m.inspecting:
			// Read-only inspection: back/cancel keys close it; up/down scroll
			// the content when it overflows.
			switch msg.String() {
			case "esc", "q", "d":
				m.inspecting = false
				m.detailScroll = 0
				m.list.SetHeight(m.naturalH)
			case "up", "k":
				if m.detailScroll > 0 {
					m.detailScroll--
				}
			case "down", "j":
				if it, ok := m.list.SelectedItem().(folderNavItem); ok {
					max := navPopupMaxScroll(m.contents[it.display], m.popupWidth())
					if m.detailScroll < max {
						m.detailScroll++
					}
				}
			}
			return m, nil
		case msg.String() == "d" && !m.focused && m.list.FilterState() != list.Filtering:
			// Folders have no contents of their own; only a highlighted
			// fragment opens the details popup.
			if it, ok := m.list.SelectedItem().(folderNavItem); ok && it.kind == itemFragment {
				m.inspecting = true
				m.detailScroll = 0
				// A short list (or an active filter) yields a frame too short
				// for the popup; grow the viewport so it fits.
				m.list.SetHeight(m.popupListHeight())
			}
			return m, nil
		case (msg.String() == "tab" || msg.String() == "shift+tab") && m.list.FilterState() == list.Unfiltered:
			m.focused = !m.focused
			return m, nil
		case msg.String() == "enter" && m.focused:
			m.result = browserDone
			m.done = true
			return m, tea.Quit
		case (msg.String() == "esc" || msg.String() == "q") && m.focused:
			m.result = browserCancel
			m.done = true
			return m, tea.Quit
		case (msg.String() == "enter" || msg.String() == " ") && !m.focused && m.list.FilterState() != list.Filtering:
			// Enter and space both toggle the highlighted row: a folder
			// expands/collapses, a fragment is picked/unpicked.
			if it, ok := m.list.SelectedItem().(folderNavItem); ok {
				switch it.kind {
				case itemFolder:
					if m.expanded[it.display] {
						delete(m.expanded, it.display)
					} else {
						m.expanded[it.display] = true
					}
					return m.rebuild()
				case itemFragment:
					if m.picked[it.display] {
						delete(m.picked, it.display)
					} else {
						m.picked[it.display] = true
					}
					return m, nil
				}
			}
		case (msg.String() == "esc" || msg.String() == "q") && !m.focused && m.list.FilterState() == list.Unfiltered:
			m.result = browserCancel
			m.done = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.width = msg.Width
		// Reserve rows for the blank separator, Done button, and help so
		// they never clip.
		m.availHeight = msg.Height - 4
		if m.availHeight < 1 {
			m.availHeight = 1
		}
		m.applyListHeight()
		if m.inspecting {
			m.list.SetHeight(m.popupListHeight())
		}
	}
	if m.focused {
		// The Done section consumes no list keys; tab/enter/esc are handled
		// above, and navigation keys are ignored while it is focused.
		return m, nil
	}
	l, cmd := m.list.Update(msg)
	m.list = l
	return m, cmd
}

func (m folderNavModel) View() string {
	if m.done {
		// The renderer moves the cursor back to the top of the frame before
		// writing this, so erasing below clears the whole folder-nav frame and
		// the next screen starts clean.
		return clearBelowFrame
	}
	// Render the list and Done sections like two huh fields in a group: the
	// focused one carries the visible border, the other a hidden one, with a
	// blank line between them and the short help below.
	var b strings.Builder
	w := m.list.Width()
	b.WriteString(sectionStyle(!m.focused).Width(w).Render(m.list.View()))
	b.WriteString("\n\n")
	btn := doneButtonBlurred
	if m.focused {
		btn = doneButtonFocused
	}
	b.WriteString(sectionStyle(m.focused).Width(w).Render(btn.Render("Done")))
	b.WriteString("\n\n")
	binds := listKeyBinds
	if m.focused {
		binds = doneKeyBinds
	}
	b.WriteString(navHelp.ShortHelpView(binds))
	if m.inspecting {
		return ui.OverlayPopup(b.String(), m.popupBox(), m.popupWidth())
	}
	return b.String()
}

// popupWidth is the window width for overlaying the details popup: the
// terminal width once a WindowSizeMsg arrived, else the list's content width.
func (m folderNavModel) popupWidth() int {
	if m.width > 0 {
		return m.width
	}
	return m.list.Width()
}

// popupBox renders the details popup for the highlighted fragment: the
// fragment's display name above its YAML contents. Folders have no contents
// and never reach here.
func (m folderNavModel) popupBox() string {
	it, ok := m.list.SelectedItem().(folderNavItem)
	if !ok {
		return ""
	}
	return navPopup(it.display, m.contents[it.display], m.popupWidth(), m.detailScroll)
}

// detailWindow is how many content lines the details popup shows at once.
const detailWindow = 10

// navPopupContentW is the popup's content width: a fixed 60 columns, shrunk to
// fit narrow terminals (border and padding take 6).
func navPopupContentW(width int) int {
	contentW := 60
	if width > 0 && contentW > width-6 {
		contentW = width - 6
	}
	if contentW < 20 {
		contentW = 20
	}
	return contentW
}

// navPopupLines wraps the popup content to the content width.
func navPopupLines(content string, contentW int) []string {
	return strings.Split(lipgloss.NewStyle().Width(contentW).Render(content), "\n")
}

// navPopupMaxScroll is the largest scroll offset that still fills the visible
// window: the wrapped lines past the window, or 0 when everything fits.
func navPopupMaxScroll(content string, width int) int {
	if content == "" {
		return 0
	}
	lines := navPopupLines(content, navPopupContentW(width))
	if len(lines) <= detailWindow {
		return 0
	}
	return len(lines) - detailWindow
}

// navPopup renders a catalog entry's details popup: the entry name above its
// YAML contents, wrapped to a fixed content width and scrolled to offset when
// the contents overflow (with a scrollbar and an ↑/↓ hint), plus an esc-close
// hint. Mirrors the approval prompt's details popup (same style, window, and
// centered overlay).
func navPopup(name, content string, width, offset int) string {
	if content == "" {
		return ""
	}
	contentW := navPopupContentW(width)
	lines := navPopupLines(content, contentW)
	scroll := len(lines) > detailWindow
	start := offset
	if scroll && start+detailWindow > len(lines) {
		start = len(lines) - detailWindow
	}
	if start < 0 {
		start = 0
	}
	win := lines[start:min(start+detailWindow, len(lines))]
	bar := newNavPopupScroll(len(lines), start, len(win))
	rendered := make([]string, len(win))
	for i, l := range win {
		rendered[i] = l + bar.cell(i)
	}
	hint := "esc close"
	if scroll {
		hint = "↑/↓ scroll · " + hint
	}
	body := name + "\n\n" + strings.Join(rendered, "\n") + "\n\n" + hint
	return navPopupStyle.Width(contentW + bar.width() + 4).Render(body)
}

// navPopupScroll is a 1-column scrollbar for the details popup: a thumb sized
// and positioned by the visible window within the full content, mirroring the
// approval list's scrollbar. It is hidden when everything fits.
type navPopupScroll struct {
	start, end int
	active     bool
}

func newNavPopupScroll(total, start, win int) navPopupScroll {
	if total <= win {
		return navPopupScroll{}
	}
	// Fixed 1-row thumb that tracks the window's position in the content.
	pos := 0
	if total > win {
		pos = start * (win - 1) / (total - win)
	}
	return navPopupScroll{start: pos, end: pos + 1, active: true}
}

func (s navPopupScroll) width() int {
	if !s.active {
		return 0
	}
	return 1
}

func (s navPopupScroll) cell(row int) string {
	if !s.active {
		return ""
	}
	if row >= s.start && row < s.end {
		return navThumb.Render("█")
	}
	return navTrack.Render("│")
}

// runFolderNav shows the folder-navigation screen and returns the picked
// fragment display names, sorted. Cancelling returns errSelectionCancelled so the
// wizard aborts like the huh prompts do.
func runFolderNav(nav fragmentNav, contents map[string]string, stdin io.Reader, stdout io.Writer) ([]string, error) {
	p := tea.NewProgram(newFolderNavModel("Fragments", nav, contents), tea.WithInput(stdin), tea.WithOutput(stdout))
	model, err := p.Run()
	if err != nil {
		return nil, err
	}
	fm := model.(folderNavModel)
	if fm.result == browserCancel {
		return nil, errSelectionCancelled
	}
	return finishPicked(fm.picked), nil
}
