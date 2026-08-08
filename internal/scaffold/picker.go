package scaffold

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jgillich/tpd/internal/ui"
)

// pickerKeyBinds are the help bindings for the single-select picker and the
// new-profile form's list section, in huh's "↑ up • ↓ down • …" short-help
// format.
var pickerKeyBinds = []key.Binding{
	key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑", "up")),
	key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓", "down")),
	key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "details")),
	key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "cancel")),
}

// nameKeyBinds are the help bindings while the new-profile name input is
// focused: tab/enter move to the base list, esc cancels.
var nameKeyBinds = []key.Binding{
	key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "base")),
	key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "base")),
	key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "cancel")),
}

// pickerItem is one row of a single-select picker: the rendered label (with a
// description appended when one exists) and the value it selects.
type pickerItem struct {
	label string
	value string
}

func (i pickerItem) FilterValue() string { return i.label + " " + i.value }

// pickerDelegate renders rows like huh's select: a tight single line with a
// fuchsia cursor on the highlighted row.
type pickerDelegate struct {
	list.DefaultDelegate
}

func (d pickerDelegate) Height() int  { return 1 }
func (d pickerDelegate) Spacing() int { return 0 }

func (d pickerDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it := item.(pickerItem)
	cursor := "  "
	if index == m.Index() {
		cursor = folderCursor.Render(">") + " "
	}
	label := folderLabelDim
	if index == m.Index() {
		label = folderLabel
	}
	fmt.Fprintf(w, "%s%s", cursor, label.Render(it.label))
}

// pickerModel is a bubbletea single-select list of catalog entries: enter
// selects the highlighted row, d opens a details popup with its YAML contents,
// esc/q cancel. Mirrors the folder-nav model's list chrome and the approval
// prompt's details popup.
type pickerModel struct {
	list         list.Model
	contents     map[string]string // entry display name → YAML contents for the details popup
	naturalH     int               // list viewport height with no details popup open
	inspecting   bool
	detailScroll int // scroll offset into the details popup's content
	width        int // window width, min 0 until a WindowSizeMsg arrives
	done         bool
	cancelled    bool
	result       string
}

func newPickerModel(title string, options []pickerItem, contents map[string]string) pickerModel {
	d := pickerDelegate{}
	d.ShowDescription = false
	items := make([]list.Item, len(options))
	for i, o := range options {
		items[i] = o
	}
	l := list.New(items, d, 80, len(items)+1)
	l.Title = title
	l.Styles.Title = folderTitle
	l.Styles.TitleBar = lipgloss.NewStyle()
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.SetShowTitle(true)
	// Quit is handled by this model so cancelling is distinguishable from a
	// stray ctrl+c; esc/q clear an applied filter before cancelling.
	l.KeyMap.Quit = key.NewBinding(key.WithDisabled())
	return pickerModel{list: l, contents: contents, width: 80, naturalH: len(items) + 1}
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.String() == "ctrl+c":
			m.cancelled = true
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
				if it, ok := m.list.SelectedItem().(pickerItem); ok {
					max := navPopupMaxScroll(m.contents[it.value], m.popupWidth())
					if m.detailScroll < max {
						m.detailScroll++
					}
				}
			}
			return m, nil
		case (msg.String() == "esc" || msg.String() == "q") && m.list.FilterState() == list.Unfiltered:
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		case msg.String() == "enter" && m.list.FilterState() != list.Filtering:
			if it, ok := m.list.SelectedItem().(pickerItem); ok {
				m.result = it.value
				m.done = true
				return m, tea.Quit
			}
		case msg.String() == "d" && m.list.FilterState() != list.Filtering:
			if it, ok := m.list.SelectedItem().(pickerItem); ok {
				if _, has := m.contents[it.value]; has {
					m.inspecting = true
					m.detailScroll = 0
					// A short list (or an active filter) yields a frame too
					// short for the popup; grow the viewport so it fits.
					m.list.SetHeight(m.popupListHeight())
				}
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.width = msg.Width
		m.setHeights(msg.Height)
		if m.inspecting {
			m.list.SetHeight(m.popupListHeight())
		}
	}
	l, cmd := m.list.Update(msg)
	m.list = l
	return m, cmd
}

// setHeights clamps the list viewport to the window minus the chrome below
// (help and one trailing blank line), recording the natural height so the
// details popup can grow it and restore it.
func (m *pickerModel) setHeights(height int) {
	avail := height - 3
	if avail < 1 {
		avail = 1
	}
	h := len(m.list.VisibleItems()) + 1
	if h > avail {
		h = avail
	}
	if h < 1 {
		h = 1
	}
	m.naturalH = h
	m.list.SetHeight(h)
}

// popupListHeight is the list viewport height that makes the frame tall enough
// to fully render the details popup. The frame is list + blank + help + blank,
// so it needs the popup's height minus that 3-row overhead.
func (m pickerModel) popupListHeight() int {
	need := len(strings.Split(m.popupBox(), "\n")) - 3
	if need < 1 {
		need = 1
	}
	if need < m.naturalH {
		return m.naturalH
	}
	return need
}

func (m pickerModel) View() string {
	if m.done {
		return clearBelowFrame
	}
	var b strings.Builder
	b.WriteString(sectionStyle(true).Width(m.list.Width()).Render(m.list.View()))
	b.WriteString("\n\n")
	b.WriteString(navHelp.ShortHelpView(pickerKeyBinds))
	b.WriteString("\n")
	if m.inspecting {
		return ui.OverlayPopup(b.String(), m.popupBox(), m.popupWidth())
	}
	return b.String()
}

func (m pickerModel) popupWidth() int {
	if m.width > 0 {
		return m.width
	}
	return m.list.Width()
}

// popupBox renders the details popup for the highlighted entry. Entries with
// no YAML contents ("New") never open one.
func (m pickerModel) popupBox() string {
	it, ok := m.list.SelectedItem().(pickerItem)
	if !ok {
		return ""
	}
	return navPopup(it.value, m.contents[it.value], m.popupWidth(), m.detailScroll)
}

// runPicker shows a single-select list and returns the selected value;
// cancelling returns errSelectionCancelled so the wizard aborts.
func runPicker(title string, options []pickerItem, contents map[string]string, stdin io.Reader, stdout io.Writer) (string, error) {
	p := tea.NewProgram(newPickerModel(title, options, contents), tea.WithInput(stdin), tea.WithOutput(stdout))
	model, err := p.Run()
	if err != nil {
		return "", err
	}
	pm := model.(pickerModel)
	if pm.cancelled {
		return "", errSelectionCancelled
	}
	return pm.result, nil
}

// newProfileModel is the wizard's "new profile" screen: a name input and a
// single-select base list on one frame, switched with tab like the folder-nav's
// list/Done sections. Enter advances from the name input to the list and
// submits from the list; d opens the highlighted base's details popup.
type newProfileModel struct {
	name         textinput.Model
	list         list.Model
	contents     map[string]string // base display name → YAML contents for the details popup
	naturalH     int               // base list viewport height with no details popup open
	focused      bool              // true: base list focused; false: name input focused
	inspecting   bool
	detailScroll int // scroll offset into the details popup's content
	width        int // window width, min 0 until a WindowSizeMsg arrives
	done         bool
	cancelled    bool
	nameResult   string
	baseResult   string
}

func newNewProfileModel(options []pickerItem, contents map[string]string) newProfileModel {
	name := textinput.New()
	name.Prompt = "> "
	name.Placeholder = "my-profile"
	name.CharLimit = 64
	name.Focus()
	d := pickerDelegate{}
	d.ShowDescription = false
	items := make([]list.Item, len(options))
	for i, o := range options {
		items[i] = o
	}
	l := list.New(items, d, 80, len(items)+1)
	l.Title = "Extend a base profile"
	l.Styles.Title = folderTitle
	l.Styles.TitleBar = lipgloss.NewStyle()
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.SetShowTitle(true)
	l.KeyMap.Quit = key.NewBinding(key.WithDisabled())
	return newProfileModel{name: name, list: l, contents: contents, width: 80, naturalH: len(items) + 1}
}

func (m newProfileModel) Init() tea.Cmd { return textinput.Blink }

func (m newProfileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.String() == "ctrl+c":
			m.cancelled = true
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
				if it, ok := m.list.SelectedItem().(pickerItem); ok {
					max := navPopupMaxScroll(m.contents[it.value], m.popupWidth())
					if m.detailScroll < max {
						m.detailScroll++
					}
				}
			}
			return m, nil
		case (msg.String() == "tab" || msg.String() == "shift+tab") && m.list.FilterState() == list.Unfiltered:
			m.focused = !m.focused
			if m.focused {
				m.name.Blur()
			} else {
				m.name.Focus()
			}
			return m, nil
		case (msg.String() == "esc" || msg.String() == "q") && m.list.FilterState() == list.Unfiltered:
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		case !m.focused:
			// Name input focused: enter moves to the base list; every other
			// key falls through to the textinput update below.
			if msg.String() == "enter" {
				m.focused = true
				m.name.Blur()
				return m, nil
			}
		case msg.String() == "enter" && m.list.FilterState() != list.Filtering:
			if it, ok := m.list.SelectedItem().(pickerItem); ok {
				m.nameResult = m.name.Value()
				m.baseResult = it.value
				m.done = true
				return m, tea.Quit
			}
		case msg.String() == "d" && m.list.FilterState() != list.Filtering:
			if it, ok := m.list.SelectedItem().(pickerItem); ok {
				if _, has := m.contents[it.value]; has {
					m.inspecting = true
					m.detailScroll = 0
					// A short list (or an active filter) yields a frame too
					// short for the popup; grow the viewport so it fits.
					m.list.SetHeight(m.popupListHeight())
				}
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		nameW := msg.Width - 2
		if nameW < 1 {
			nameW = 1
		}
		m.name.Width = nameW
		m.width = msg.Width
		m.setHeights(msg.Height)
		if m.inspecting {
			m.list.SetHeight(m.popupListHeight())
		}
	}
	if m.focused {
		l, cmd := m.list.Update(msg)
		m.list = l
		return m, cmd
	}
	var cmd tea.Cmd
	m.name, cmd = m.name.Update(msg)
	return m, cmd
}

// setHeights clamps the base list viewport to the window minus the chrome
// above (name section) and below (help and one trailing blank line), recording
// the natural height so the details popup can grow it and restore it.
func (m *newProfileModel) setHeights(height int) {
	avail := height - 7
	if avail < 1 {
		avail = 1
	}
	h := len(m.list.VisibleItems()) + 1
	if h > avail {
		h = avail
	}
	if h < 1 {
		h = 1
	}
	m.naturalH = h
	m.list.SetHeight(h)
}

// popupListHeight is the base list viewport height that makes the frame tall
// enough to fully render the details popup. The frame is name title + input +
// blank + base title + list + blank + help + blank, so it needs the popup's
// height minus that 7-row overhead.
func (m newProfileModel) popupListHeight() int {
	need := len(strings.Split(m.popupBox(), "\n")) - 7
	if need < 1 {
		need = 1
	}
	if need < m.naturalH {
		return m.naturalH
	}
	return need
}

func (m newProfileModel) View() string {
	if m.done {
		return clearBelowFrame
	}
	var b strings.Builder
	w := m.list.Width()
	b.WriteString(folderTitle.Render("New profile name"))
	b.WriteString("\n")
	b.WriteString(sectionStyle(!m.focused).Width(w).Render(m.name.View()))
	b.WriteString("\n\n")
	b.WriteString(sectionStyle(m.focused).Width(w).Render(m.list.View()))
	b.WriteString("\n\n")
	binds := pickerKeyBinds
	if !m.focused {
		binds = nameKeyBinds
	}
	b.WriteString(navHelp.ShortHelpView(binds))
	b.WriteString("\n")
	if m.inspecting {
		return ui.OverlayPopup(b.String(), m.popupBox(), m.popupWidth())
	}
	return b.String()
}

func (m newProfileModel) popupWidth() int {
	if m.width > 0 {
		return m.width
	}
	return m.list.Width()
}

// popupBox renders the details popup for the highlighted base.
func (m newProfileModel) popupBox() string {
	it, ok := m.list.SelectedItem().(pickerItem)
	if !ok {
		return ""
	}
	return navPopup(it.value, m.contents[it.value], m.popupWidth(), m.detailScroll)
}

// runNewProfile shows the new-profile screen (name + base) and returns the
// typed name and the selected base display name; cancelling returns
// errSelectionCancelled so the wizard aborts.
func runNewProfile(options []pickerItem, contents map[string]string, stdin io.Reader, stdout io.Writer) (string, string, error) {
	p := tea.NewProgram(newNewProfileModel(options, contents), tea.WithInput(stdin), tea.WithOutput(stdout))
	model, err := p.Run()
	if err != nil {
		return "", "", err
	}
	nm := model.(newProfileModel)
	if nm.cancelled {
		return "", "", errSelectionCancelled
	}
	return nm.nameResult, nm.baseResult, nil
}
