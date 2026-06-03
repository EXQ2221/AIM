package queryrouter

import "testing"

func TestPlanningInputNormalizedDefaults(t *testing.T) {
	input := PlanningInput{
		UserQuery: "  总结这本书的核心观点  ",
		SelectedTargets: []Target{
			{ID: " doc_1 ", Type: "Document", Title: "A"},
			{ID: "doc_1", Type: "document", Title: "A"},
		},
	}

	got := input.Normalized()
	if got.UserQuery != "总结这本书的核心观点" {
		t.Fatalf("unexpected user query: %q", got.UserQuery)
	}
	if len(got.SelectedTargets) != 1 || got.SelectedTargets[0].ID != "doc_1" || got.SelectedTargets[0].Type != "document" {
		t.Fatalf("unexpected targets: %+v", got.SelectedTargets)
	}
	if !got.AvailableSpaces.SelectedDocuments || !got.AvailableSpaces.Metadata || !got.AvailableSpaces.Mixed {
		t.Fatalf("available spaces defaults not applied: %+v", got.AvailableSpaces)
	}
	if !got.Capabilities.CanLookup || !got.Capabilities.CanFullReadDocument || got.Capabilities.CanExtractExactQuote {
		t.Fatalf("capability defaults not applied: %+v", got.Capabilities)
	}
}

func TestPlanNormalizedTurnsUnsupportedForExactQuoteWithoutCapability(t *testing.T) {
	input := PlanningInput{
		UserQuery: "总结并给原句",
		SelectedTargets: []Target{
			{ID: "doc_1", Type: "document"},
		},
		Capabilities: Capabilities{
			CanLookup:                  true,
			CanFullReadDocument:        true,
			CanSynthesizeMultiDocument: true,
			CanExtractExactQuote:       false,
			CanControlBindings:         true,
		},
	}

	plan := Plan{
		Family:       FamilyRead,
		SourceSpace:  SourceSpaceSelectedDocuments,
		Scope:        ScopeDocument,
		ReadDepth:    ReadDepthFullRead,
		OutputMode:   OutputModeSummary,
		EvidenceMode: EvidenceModeExactQuote,
		Targets:      []string{"doc_1"},
	}.Normalized(input)

	if plan.Family != FamilyUnsupported {
		t.Fatalf("expected unsupported, got %s", plan.Family)
	}
	if !plan.Constraints.StrictQuoteRequired {
		t.Fatalf("expected strict quote requirement")
	}
	if plan.Reason == "" {
		t.Fatal("expected unsupported reason")
	}
}

func TestPlanNormalizedUsesSelectedDocumentsTargetsByDefault(t *testing.T) {
	input := PlanningInput{
		UserQuery: "总结这本书",
		SelectedTargets: []Target{
			{ID: "doc_1", Type: "document"},
			{ID: "doc_2", Type: "document"},
		},
	}

	plan := Plan{
		Family:      FamilyRead,
		SourceSpace: SourceSpaceSelectedDocuments,
	}.Normalized(input)

	if len(plan.Targets) != 2 || plan.Targets[0] != "doc_1" || plan.Targets[1] != "doc_2" {
		t.Fatalf("unexpected default targets: %+v", plan.Targets)
	}
	if plan.Scope != ScopeDocument {
		t.Fatalf("unexpected scope: %s", plan.Scope)
	}
	if plan.ReadDepth != ReadDepthFullRead {
		t.Fatalf("unexpected read depth: %s", plan.ReadDepth)
	}
}
