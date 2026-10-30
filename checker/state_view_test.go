package checker

import (
	"strings"
	"testing"
)

func TestUpstreamStateViewReducedMatrix(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-state-view-reduced-matrix.jsonl", 288)
}

func TestUpstreamStateViewMultiMatrix(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-state-view-multi-matrix.jsonl", 384)
}

func TestUpstreamStateViewYMatrix(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-state-view-y-matrix.jsonl", 512)
}

func TestUpstreamSymmetryGeneralMatrix(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-symmetry-general-matrix.jsonl", 144)
}

func TestMergedViewGeneralFairnessRemainsUnsupported(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x,y\nvars == <<x,y>>\nInit == x=0 /\\ y=0\nNext == x'=0 /\\ y' \\in {0,1}\nSpec == Init /\\ [][Next]_vars /\\ WF_vars(Next)\nV == x\nP == <>(x=0 /\\ []<>(x=1))\n===="
	_, err := Check(source, "SPECIFICATION Spec VIEW V PROPERTY P CHECK_DEADLOCK FALSE")
	if err == nil || !strings.Contains(err.Error(), "distinct concrete successors are not yet supported") {
		t.Fatalf("got %v; want the reduced fairness unsupported error", err)
	}
}
