package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// OverlayPopup splices a popup box onto a base frame of the given width,
// centered vertically. The base rows the popup covers become plain text
// outside the popup (the popup's background masks them); every other row keeps
// its styling.
func OverlayPopup(base string, popup string, width int) string {
	baseLines := PadLinesTo(strings.Split(base, "\n"), width)
	popupLines := strings.Split(popup, "\n")
	popupW := 0
	for _, l := range popupLines {
		if w := lipgloss.Width(l); w > popupW {
			popupW = w
		}
	}
	popupH := len(popupLines)
	x := (width - popupW) / 2
	if x < 0 {
		x = 0
	}
	y := (len(baseLines) - popupH) / 2
	if y < 0 {
		y = 0
	}
	for k := 0; k < popupH && y+k < len(baseLines); k++ {
		plain := []rune(ansi.Strip(baseLines[y+k]))
		var b strings.Builder
		if x > 0 && x <= len(plain) {
			b.WriteString(string(plain[:x]))
		}
		b.WriteString(PadToWidth(popupLines[k], popupW))
		if x+popupW < len(plain) {
			b.WriteString(string(plain[x+popupW:]))
		}
		baseLines[y+k] = b.String()
	}
	return strings.Join(baseLines, "\n")
}

func PadToWidth(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func PadLinesTo(lines []string, w int) []string {
	for i, l := range lines {
		lines[i] = PadToWidth(l, w)
	}
	return lines
}
