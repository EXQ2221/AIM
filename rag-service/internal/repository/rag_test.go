package repository

import "testing"

func TestBuildSearchTermsAddsOverlapFragments(t *testing.T) {
	terms := BuildSearchTerms("方向导数公式")

	for _, want := range []string{"方向", "向导", "导数", "公式"} {
		if !containsTerm(terms, want) {
			t.Fatalf("BuildSearchTerms missing %q in %v", want, terms)
		}
	}
	if containsTerm(terms, "方") {
		t.Fatalf("BuildSearchTerms should ignore single-rune fragments: %v", terms)
	}
}

func TestCalcTermOverlapScoreRewardsSharedTopic(t *testing.T) {
	left := BuildSearchTerms("方向导数公式")
	right := BuildSearchTerms("方向导数与梯度")
	score := CalcTermOverlapScore(left, right)
	if score < 0.4 {
		t.Fatalf("CalcTermOverlapScore = %v, want >= 0.4", score)
	}

	weak := CalcTermOverlapScore(BuildSearchTerms("那具体的公式有没有"), right)
	if weak >= score {
		t.Fatalf("weak overlap = %v, want < %v", weak, score)
	}
}

func containsTerm(terms []string, want string) bool {
	for _, term := range terms {
		if term == want {
			return true
		}
	}
	return false
}
