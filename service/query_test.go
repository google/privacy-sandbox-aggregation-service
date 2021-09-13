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
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func TestExpansionConfigReadWrite(t *testing.T) {
	tmpDir, err := ioutil.TempDir("/tmp", "test-config")
	if err != nil {
		t.Fatalf("failed to create temp dir: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &ExpansionConfig{
		PrefixLengths:               []int32{1, 2, 3},
		PrivacyBudgetPerPrefix:      []float64{0.2, 0.5, 0.3},
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

func TestMergePartialHistogram(t *testing.T) {
	partial1 := map[uint64]*pb.PartialAggregationDpf{
		0: &pb.PartialAggregationDpf{PartialSum: 1},
		1: &pb.PartialAggregationDpf{PartialSum: 1},
		2: &pb.PartialAggregationDpf{PartialSum: 1},
		3: &pb.PartialAggregationDpf{PartialSum: 1},
	}
	partial2 := map[uint64]*pb.PartialAggregationDpf{
		0: &pb.PartialAggregationDpf{PartialSum: 0},
		1: &pb.PartialAggregationDpf{PartialSum: 1},
		2: &pb.PartialAggregationDpf{PartialSum: 2},
		3: &pb.PartialAggregationDpf{PartialSum: 3},
	}

	got, err := mergePartialHistogram(partial1, partial2)
	if err != nil {
		t.Fatal(err)
	}

	want := []dpfaggregator.CompleteHistogram{
		{Index: 0, Sum: 1},
		{Index: 1, Sum: 2},
		{Index: 2, Sum: 3},
		{Index: 3, Sum: 4},
	}
	if diff := cmp.Diff(want, got, cmpopts.SortSlices(func(a, b dpfaggregator.CompleteHistogram) bool { return a.Index < b.Index })); diff != "" {
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
	return ioutils.WriteLines(ctx, lines, filename)
}

func TestGetNextLevelRequest(t *testing.T) {
	config := &ExpansionConfig{
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
	if err := WriteExpansionConfigFile(ctx, config, expandConfigURI); err != nil {
		t.Fatal(err)
	}

	queryID := "unit-test"
	partialFile1 := ioutils.JoinPath(sharedDir1, fmt.Sprintf("%s_%s_%d", queryID, DefaultPartialResultFile, 0))
	if err := writePartialHistogram(ctx, partialFile1, partial1); err != nil {
		t.Fatal(err)
	}
	partialFile2 := ioutils.JoinPath(sharedDir2, fmt.Sprintf("%s_%s_%d", queryID, DefaultPartialResultFile, 0))
	if err := writePartialHistogram(ctx, partialFile2, partial2); err != nil {
		t.Fatal(err)
	}

	request := &AggregateRequest{
		ExpandConfigURI: expandConfigURI,
		QueryID:         queryID,
		TotalEpsilon:    0.5,
	}
	type aggParams struct {
		Request   *AggregateRequest
		SumParams *pb.IncrementalDpfParameters
		Prefixes  *pb.HierarchicalPrefixes
	}
	for i, want := range []*aggParams{
		{
			Request: &AggregateRequest{
				ExpandConfigURI: expandConfigURI,
				QueryID:         queryID,
				TotalEpsilon:    0.5,
				Level:           0,
				SumParamsURI:    ioutils.JoinPath(sharedDir1, fmt.Sprintf("%s_%s_%d", queryID, DefaultSumParamsFile, 0)),
				PrefixesURI:     ioutils.JoinPath(sharedDir1, fmt.Sprintf("%s_%s_%d", queryID, DefaultPrefixesFile, 0)),
			},
			SumParams: &pb.IncrementalDpfParameters{
				Params: []*dpfpb.DpfParameters{
					{LogDomainSize: 1, ElementBitsize: elementBitSize},
				},
			},
			Prefixes: &pb.HierarchicalPrefixes{
				Prefixes: []*pb.DomainPrefixes{
					{},
				},
			},
		},
		{
			Request: &AggregateRequest{
				ExpandConfigURI: expandConfigURI,
				QueryID:         queryID,
				TotalEpsilon:    0.5,
				Level:           1,
				SumParamsURI:    ioutils.JoinPath(sharedDir1, fmt.Sprintf("%s_%s_%d", queryID, DefaultSumParamsFile, 1)),
				PrefixesURI:     ioutils.JoinPath(sharedDir1, fmt.Sprintf("%s_%s_%d", queryID, DefaultPrefixesFile, 1)),
			},
			SumParams: &pb.IncrementalDpfParameters{
				Params: []*dpfpb.DpfParameters{
					{LogDomainSize: 1, ElementBitsize: elementBitSize},
					{LogDomainSize: 2, ElementBitsize: elementBitSize},
				},
			},
			Prefixes: &pb.HierarchicalPrefixes{
				Prefixes: []*pb.DomainPrefixes{
					{},
					{Prefix: []uint64{1}},
				},
			},
		},
		{},
	} {
		got, err := GetRequestParams(ctx, config, request, sharedDir1, sharedDir2)
		if err != nil {
			if err.Error() == "expect request level <= final level 1, got 2" {
				continue
			}
			t.Fatal(err)
		}
		if diff := cmp.Diff(want.Request, got); diff != "" {
			t.Fatalf("request mismatch for i=%d (-want +got):\n%s", i, diff)
		}

		gotSumParams, err := cryptoio.ReadDPFParameters(ctx, got.SumParamsURI)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want.SumParams, gotSumParams, protocmp.Transform()); diff != "" {
			t.Errorf("sum params mismatch (-want +got):\n%s", diff)
		}

		gotPrefixes, err := cryptoio.ReadPrefixes(ctx, got.PrefixesURI)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want.Prefixes, gotPrefixes, protocmp.Transform()); diff != "" {
			t.Errorf("prefixes mismatch (-want +got):\n%s", diff)
		}
		*request = *got
		request.Level++
	}
}
