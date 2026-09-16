package gimblelint

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "setchecks", "dispatch", "ordinary", "promptchecks", "github.com/tylergannon/gimble/cmd")
}
