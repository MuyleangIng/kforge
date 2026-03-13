package commands

import (
	"strings"
	"unicode/utf8"
)

const (
	boxInnerWidth   = 60
	boxContentWidth = boxInnerWidth - 2
	boxLabelWidth   = 10
	boxValueGap     = 2
)

func boxTop() string {
	return cCyan + "╔" + strings.Repeat("═", boxInnerWidth) + "╗" + cReset
}

func boxBottom() string {
	return cCyan + "╚" + strings.Repeat("═", boxInnerWidth) + "╝" + cReset
}

func boxDivider() string {
	return cCyan + "║" + strings.Repeat("─", boxInnerWidth) + "║" + cReset
}

func boxLine(content string) string {
	visible := visibleWidth(content)
	pad := boxContentWidth - visible
	if pad < 0 {
		pad = 0
	}
	return cCyan + "║" + cReset + " " + content + strings.Repeat(" ", pad) + " " + cCyan + "║" + cReset
}

func boxTitleLine(left, leftStyle, right, rightStyle string) string {
	left = truncatePlain(left, boxContentWidth)
	if strings.TrimSpace(right) == "" {
		return boxLine(styleText(left, leftStyle))
	}

	minGap := 2
	maxRight := boxContentWidth - visibleWidth(left) - minGap
	if maxRight < 0 {
		maxRight = 0
	}
	right = truncatePlain(right, maxRight)

	leftStyled := styleText(left, leftStyle)
	rightStyled := styleText(right, rightStyle)
	gap := boxContentWidth - visibleWidth(leftStyled) - visibleWidth(rightStyled)
	if gap < minGap {
		gap = minGap
	}

	return boxLine(leftStyled + strings.Repeat(" ", gap) + rightStyled)
}

func boxKeyValueLines(label string, values []string, valueStyle string) []string {
	if len(values) == 0 {
		values = []string{""}
	}

	label = truncatePlain(label, boxLabelWidth)
	labelText := padRightPlain(label, boxLabelWidth)
	continuation := strings.Repeat(" ", boxLabelWidth)
	valueWidth := boxContentWidth - boxLabelWidth - boxValueGap
	if valueWidth < 8 {
		valueWidth = 8
	}

	lines := make([]string, 0, len(values))
	first := true
	for _, value := range values {
		segments := wrapText(value, valueWidth)
		if len(segments) == 0 {
			segments = []string{""}
		}
		for _, segment := range segments {
			currentLabel := continuation
			if first {
				currentLabel = labelText
			}
			lines = append(lines, boxLine(
				styleText(currentLabel, cDim)+"  "+styleText(segment, valueStyle),
			))
			first = false
		}
	}

	return lines
}

func styleText(text, style string) string {
	if text == "" || style == "" {
		return text
	}
	return style + text + cReset
}

func visibleWidth(s string) int {
	return utf8.RuneCountInString(stripANSI(s))
}

func truncatePlain(text string, width int) string {
	if width <= 0 {
		return ""
	}

	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= width {
		return string(runes)
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

func padRightPlain(text string, width int) string {
	if width <= 0 {
		return ""
	}
	text = truncatePlain(text, width)
	pad := width - visibleWidth(text)
	if pad < 0 {
		pad = 0
	}
	return text + strings.Repeat(" ", pad)
}

func wrapText(text string, width int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{""}
	}
	if width <= 0 {
		return []string{text}
	}

	lines := []string{}
	for _, paragraph := range strings.Split(text, "\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		current := ""
		for _, word := range strings.Fields(paragraph) {
			if current == "" {
				if visibleWidth(word) <= width {
					current = word
					continue
				}
				chunks := splitLongToken(word, width)
				lines = append(lines, chunks[:len(chunks)-1]...)
				current = chunks[len(chunks)-1]
				continue
			}

			candidate := current + " " + word
			if visibleWidth(candidate) <= width {
				current = candidate
				continue
			}

			lines = append(lines, current)
			if visibleWidth(word) <= width {
				current = word
				continue
			}
			chunks := splitLongToken(word, width)
			lines = append(lines, chunks[:len(chunks)-1]...)
			current = chunks[len(chunks)-1]
		}

		if current != "" {
			lines = append(lines, current)
		}
	}

	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func splitLongToken(token string, width int) []string {
	if width <= 0 {
		return []string{token}
	}

	runes := []rune(token)
	if len(runes) <= width {
		return []string{token}
	}

	chunks := []string{}
	for len(runes) > width {
		cut := preferredBreak(runes, width)
		chunks = append(chunks, string(runes[:cut]))
		runes = runes[cut:]
	}
	if len(runes) > 0 {
		chunks = append(chunks, string(runes))
	}
	return chunks
}

func preferredBreak(runes []rune, width int) int {
	if len(runes) <= width {
		return len(runes)
	}

	start := width - 12
	if start < 1 {
		start = 1
	}
	for i := width; i >= start; i-- {
		switch runes[i-1] {
		case '/', ':', '@', '-', '_', '.':
			return i
		}
	}
	return width
}

func displayProgressStyle(style string) string {
	style = strings.TrimSpace(style)
	if style == "" {
		return "Auto"
	}
	return strings.ToUpper(style[:1]) + strings.ToLower(style[1:])
}
