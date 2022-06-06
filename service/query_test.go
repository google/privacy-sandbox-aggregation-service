package query

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

func TestHierarchicalConfigReadWrite(t *testing.T) {
	tmpDir, err := ioutil.TempDir("/tmp", "test-config")
	if err != nil {
		t.Fatalf("failed to create temp dir: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &HierarchicalConfig{
		PrefixLengths:               []int32{1, 2, 3},
		PrivacyBudgetPerPrefix:      []float64{0.2, 0.5, 0.3},
		ExpansionThresholdPerPrefix: []uint64{4, 5, 6},
	}
	ctx := context.Background()
	configFile := path.Join(tmpDir, "config_file")
	if err := WriteHierarchicalConfigFile(ctx, config, configFile); err != nil {
		t.Fatal(err)
	}
	got, err := ReadHierarchicalConfigFile(ctx, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(config, got); diff != "" {
		t.Errorf("expansion config read/write mismatch (-want +got):\n%s", diff)
	}
}

func TestGetNextNonemptyPrefixes(t *testing.T) {
	result := []dpfaggregator.CompleteHistogram{
		{Bucket: uint128.From64(1), Sum: 2},
		{Bucket: uint128.From64(2), Sum: 3},
		{Bucket: uint128.From64(3), Sum: 4},
	}
	got := getNextNonemptyPrefixes(result, 3)
	want := []uint128.Uint128{uint128.From64(2), uint128.From64(3)}
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
		{PrefixLength: 1, Histogram: []dpfaggregator.CompleteHistogram{{Bucket: uint128.From64(1), Sum: 1}}, ExpansionThreshold: 1},
	}
	resultsFile := path.Join(tmpDir, "results")
	ctx := context.Background()
	if err := WriteHierarchicalResultsFile(ctx, wantResults, resultsFile); err != nil {
		t.Fatal(err)
	}
	got, err := ReadHierarchicalResultsFile(ctx, resultsFile)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantResults, got); diff != "" {
		t.Errorf("results mismatch (-want +got):\n%s", diff)
	}
}

func writePartialHistogram(ctx context.Context, filename string, results map[uint64]*pb.PartialAggregationDpf) error {
	var lines []string
	for id, result := range results {
		b, err := proto.Marshal(result)
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%d,%s", id, base64.StdEncoding.EncodeToString(b)))
	}
	return utils.WriteLines(ctx, lines, filename)
}

func TestGetRequestExpandParams(t *testing.T) {
	config := &HierarchicalConfig{
		PrefixLengths:               []int32{1, 2},
		PrivacyBudgetPerPrefix:      []float64{0.6, 0.4},
		ExpansionThresholdPerPrefix: []uint64{2, 5},
	}
	partial1 := map[uint64]*pb.PartialAggregationDpf{
		0: &pb.PartialAggregationDpf{PartialSum: 1},
		1: &pb.PartialAggregationDpf{PartialSum: 1},
	}
	partial2 := map[uint64]*pb.PartialAggregationDpf{
		0: &pb.PartialAggregationDpf{PartialSum: 0},
		1: &pb.PartialAggregationDpf{PartialSum: 2},
	}

	tmpDir, err := ioutil.TempDir("/tmp", "test-results")
	if err != nil {
		t.Fatalf("failed to create temp dir: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	workspace := path.Join(tmpDir, "workspace")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatal(err)
	}

	sharedDir1 := path.Join(tmpDir, "sharedDir1")
	if err := os.MkdirAll(sharedDir1, 0755); err != nil {
		t.Fatal(err)
	}
	sharedDir2 := path.Join(tmpDir, "sharedDir2")
	if err := os.MkdirAll(sharedDir2, 0755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	expandConfigURI := path.Join(tmpDir, "expand_config_file.json")
	if err := WriteHierarchicalConfigFile(ctx, config, expandConfigURI); err != nil {
		t.Fatal(err)
	}

	queryID := "unit-test"
	partialFile1 := utils.JoinPath(sharedDir1, fmt.Sprintf("%s_%s_%d", queryID, DefaultPartialResultFile, 0))
	if err := writePartialHistogram(ctx, partialFile1, partial1); err != nil {
		t.Fatal(err)
	}
	partialFile2 := utils.JoinPath(sharedDir2, fmt.Sprintf("%s_%s_%d", queryID, DefaultPartialResultFile, 0))
	if err := writePartialHistogram(ctx, partialFile2, partial2); err != nil {
		t.Fatal(err)
	}

	request := &AggregateRequest{
		ExpandConfigURI: expandConfigURI,
		QueryID:         queryID,
		TotalEpsilon:    0.5,
	}
	type aggParams struct {
		ExpandParamsURI string
		ExpandParams    *dpfaggregator.ExpandParameters
	}
	for _, want := range []*aggParams{
		{
			ExpandParamsURI: utils.JoinPath(workspace, fmt.Sprintf("%s_%s_%d", queryID, DefaultExpandParamsFile, 0)),
			ExpandParams: &dpfaggregator.ExpandParameters{
				Level:         0,
				PreviousLevel: -1,
			},
		},
		{
			ExpandParamsURI: utils.JoinPath(workspace, fmt.Sprintf("%s_%s_%d", queryID, DefaultExpandParamsFile, 1)),
			ExpandParams: &dpfaggregator.ExpandParameters{
				Level:         1,
				Prefixes:      []uint128.Uint128{uint128.From64(1)},
				PreviousLevel: 0,
			},
		},
		{},
	} {
		gotExpandParamsFile, err := GetRequestExpandParamsURI(ctx, config, request, workspace, sharedDir1, sharedDir2)
		if err != nil {
			if err.Error() == "expect request level <= final level 1, got 2" {
				continue
			}
			t.Fatal(err)
		}
		if gotExpandParamsFile != want.ExpandParamsURI {
			t.Fatalf("expect expand params URI %q, got %q", want.ExpandParamsURI, gotExpandParamsFile)
		}

		gotExpandParams, err := dpfaggregator.ReadExpandParameters(ctx, gotExpandParamsFile)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(want.ExpandParams, gotExpandParams); diff != "" {
			t.Errorf("expand params mismatch (-want +got):\n%s", diff)
		}
		request.QueryLevel++
	}
}
