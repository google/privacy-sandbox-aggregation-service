package query

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
)

func TestExpansionConfigReadWrite(t *testing.T) {
	tmpDir, err := ioutil.TempDir("/tmp", "test-config")
	if err != nil {
		t.Fatalf("failed to create temp dir: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &ExpansionConfig{
		PrefixLengths:               []int32{1, 2, 3},
		ExpansionThresholdPerPrefix: []uint64{4, 5, 6},
	}
	ctx := context.Background()
	configFile := path.Join(tmpDir, "config_file")
	if err := WriteExpansionConfigFile(ctx, config, configFile); err != nil {
		t.Fatal(err)
	}
	got, err := ReadExpansionConfigFile(ctx, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(config, got); diff != "" {
		t.Errorf("expansion config read/write mismatch (-want +got):\n%s", diff)
	}
}

func TestGetNextNonemptyPrefixes(t *testing.T) {
	result := []dpfaggregator.CompleteHistogram{
		{Index: 1, Sum: 2},
		{Index: 2, Sum: 3},
		{Index: 3, Sum: 4},
	}
	got := getNextNonemptyPrefixes(result, 3)
	want := []uint64{2, 3}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("nonempty prefixes mismatch (-want +got):\n%s", diff)
	}
}

func TestHierarchicalResultsReadWrite(t *testing.T) {
	tmpDir, err := ioutil.TempDir("/tmp", "test-results")
	if err != nil {
		t.Fatalf("failed to create temp dir: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	wantResults := []HierarchicalResult{
		{PrefixLength: 1, Histogram: []dpfaggregator.CompleteHistogram{{Index: 1, Sum: 1}}, ExpansionThreshold: 1},
	}
	resultsFile := path.Join(tmpDir, "results")
	if err := WriteHierarchicalResultsFile(wantResults, resultsFile); err != nil {
		t.Fatal(err)
	}
	got, err := ReadHierarchicalResultsFile(resultsFile)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantResults, got); diff != "" {
		t.Errorf("results mismatch (-want +got):\n%s", diff)
	}
}
