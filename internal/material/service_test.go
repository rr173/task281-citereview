package material

import "testing"

func TestSummaryHashStable(t *testing.T) {
	a := SummaryHash("  限于A省行政区域内适用本规则。  ")
	b := SummaryHash("限于A省行政区域内适用本规则。")
	if a != b {
		t.Fatalf("hash mismatch: %q vs %q", a, b)
	}
}

func TestNormalizeTextCollapsesWhitespace(t *testing.T) {
	got := normalizeText("line1\n\nline2")
	if got != "line1 line2" {
		t.Fatalf("normalize = %q", got)
	}
}
