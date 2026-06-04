package rag

import "testing"

func TestCalcTitleScoreSimpleUsesOverlapFragments(t *testing.T) {
	got := calcTitleScoreSimple("方向导数公式", "9.7 方向导数与梯度")
	if got < 0.4 {
		t.Fatalf("calcTitleScoreSimple = %v, want >= 0.4", got)
	}

	exact := calcTitleScoreSimple("方向导数与梯度", "方向导数与梯度")
	if exact != 1 {
		t.Fatalf("calcTitleScoreSimple exact match = %v, want 1", exact)
	}

	weak := calcTitleScoreSimple("那具体的公式有没有", "9.7 方向导数与梯度")
	if weak >= got {
		t.Fatalf("weak title score = %v, want < %v", weak, got)
	}
}
