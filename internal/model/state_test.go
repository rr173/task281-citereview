package model

import "testing"

func TestValidBatchTransition(t *testing.T) {
	if !ValidBatchTransition(BatchOrganizing, BatchAnalyzing) {
		t.Fatal("organizing -> analyzing should be valid")
	}
	if ValidBatchTransition(BatchPublished, BatchOrganizing) {
		t.Fatal("published -> organizing must be invalid")
	}
}

func TestValidGraphVersionTransition(t *testing.T) {
	if !ValidGraphVersionTransition(GVDraft, GVFrozen) {
		t.Fatal("draft -> frozen should be valid")
	}
	if ValidGraphVersionTransition(GVFrozen, GVDraft) {
		t.Fatal("frozen -> draft must be invalid")
	}
}
