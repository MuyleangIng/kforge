package commands

import (
	"strings"
	"testing"
)

func TestBoxLineWidthIsConsistent(t *testing.T) {
	line := stripANSI(boxLine("hello"))
	if got, want := visibleWidth(line), boxInnerWidth+2; got != want {
		t.Fatalf("boxLine width = %d, want %d: %q", got, want, line)
	}
}

func TestBoxKeyValueLinesWrapLongValues(t *testing.T) {
	lines := boxKeyValueLines("Tags", []string{
		"ghcr.io/MuyleangIng/flask-ci-demo:ba81f101bd0b41ca145d0ef6fd0f53d92aa629f4",
		"ghcr.io/MuyleangIng/flask-ci-demo:latest",
	}, cBold)

	if len(lines) < 3 {
		t.Fatalf("expected wrapped lines for long tags, got %d", len(lines))
	}

	if !strings.Contains(stripANSI(lines[0]), "Tags") {
		t.Fatalf("first line should include the label: %q", stripANSI(lines[0]))
	}

	for i, line := range lines {
		if got, want := visibleWidth(stripANSI(line)), boxInnerWidth+2; got != want {
			t.Fatalf("line %d width = %d, want %d: %q", i, got, want, stripANSI(line))
		}
	}
}

func TestBoxTitleLineWidthIsConsistent(t *testing.T) {
	line := stripANSI(boxTitleLine("KFORGE BUILD", cBold+cWhite, "dev · KhmerStack", cDim))
	if got, want := visibleWidth(line), boxInnerWidth+2; got != want {
		t.Fatalf("boxTitleLine width = %d, want %d: %q", got, want, line)
	}
}
