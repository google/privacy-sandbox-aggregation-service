package dpf_pipeline_integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"

	
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestPipeline(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := ioutil.TempDir("/tmp", "test-private")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	encryptionKeyDir := path.Join(tmpDir, "encryption_key_dir")
	if err := os.MkdirAll(encryptionKeyDir, 0755); err != nil {
		t.Fatal(err)
	}
	privateKeyURI := path.Join(encryptionKeyDir, "private_key")
	publicKeyURI := path.Join(encryptionKeyDir, "public_key")
	if err := executeCommand(ctx, "create_hybrid_key_pair",
		"--private_key_dir="+encryptionKeyDir,
		"--private_key_info_file="+privateKeyURI,
		"--public_key_info_file="+publicKeyURI,
	); err != nil {
		t.Fatal(err)
	}

	partialReportDir := path.Join(tmpDir, "partial_report_dir")
	if err := os.MkdirAll(partialReportDir, 0755); err != nil {
		t.Fatal(err)
	}
	partialReportURI1 := path.Join(partialReportDir, "encrypted_partial_report1")
	partialReportURI2 := path.Join(partialReportDir, "encrypted_partial_report2")
	testFile := "dpf_test_conversion_data.csv"
	
	if err := executeCommand(ctx, "dpf_generate_partial_report",
		"--conversion_uri="+testFile,
		"--partial_report_uri1="+partialReportURI1,
		"--partial_report_uri2="+partialReportURI2,
		"--public_keys_uri1="+publicKeyURI,
		"--public_keys_uri2="+publicKeyURI,
	); err != nil {
		t.Fatal(err)
	}

	// First-level aggregation: 2-bit prefixes.
	expandParamsDir := path.Join(tmpDir, "expand_params_dir")
	if err := os.MkdirAll(expandParamsDir, 0755); err != nil {
		t.Fatal(err)
	}
	expandParamsURI0 := path.Join(expandParamsDir, "expand_params0")
	if err := cryptoio.SaveExpandParameters(ctx, &pb.ExpandParameters{
		Levels: []int32{1},
		Prefixes: &pb.HierarchicalPrefixes{
			Prefixes: []*pb.DomainPrefixes{
				{Prefix: []uint64{}},
			},
		},
		PreviousLevel: -1,
	}, expandParamsURI0); err != nil {
		t.Fatal(err)
	}

	partialResultDir1 := path.Join(tmpDir, "partial_result_dir1")
	if err := os.MkdirAll(partialResultDir1, 0755); err != nil {
		t.Fatal(err)
	}
	workspaceDir1 := path.Join(tmpDir, "workspace_dir1")
	if err := os.MkdirAll(workspaceDir1, 0755); err != nil {
		t.Fatal(err)
	}
	partialHistogramURI01 := path.Join(partialResultDir1, "partial_histogram0")
	evaluationContextURI01 := path.Join(workspaceDir1, "evaluation_context0")
	if err := executeCommand(ctx, "dpf_aggregate_partial_report",
		"--partial_report_uri="+partialReportURI1,
		"--expand_parameters_uri="+expandParamsURI0,
		"--partial_histogram_uri="+partialHistogramURI01,
		"--evaluation_context_uri="+evaluationContextURI01,
		"--private_key_params_uri="+privateKeyURI,
	); err != nil {
		t.Fatal(err)
	}

	partialResultDir2 := path.Join(tmpDir, "partial_result_dir2")
	if err := os.MkdirAll(partialResultDir2, 0755); err != nil {
		t.Fatal(err)
	}
	workspaceDir2 := path.Join(tmpDir, "workspace_dir2")
	if err := os.MkdirAll(workspaceDir2, 0755); err != nil {
		t.Fatal(err)
	}
	partialHistogramURI02 := path.Join(partialResultDir2, "partial_histogram0")
	evaluationContextURI02 := path.Join(workspaceDir2, "evaluation_context0")
	if err := executeCommand(ctx, "dpf_aggregate_partial_report",
		"--partial_report_uri="+partialReportURI2,
		"--expand_parameters_uri="+expandParamsURI0,
		"--partial_histogram_uri="+partialHistogramURI02,
		"--evaluation_context_uri="+evaluationContextURI02,
		"--private_key_params_uri="+privateKeyURI,
	); err != nil {
		t.Fatal(err)
	}

	result01, err := dpfaggregator.ReadPartialHistogram(ctx, partialHistogramURI01)
	if err != nil {
		t.Fatal(err)
	}
	result02, err := dpfaggregator.ReadPartialHistogram(ctx, partialHistogramURI02)
	if err != nil {
		t.Fatal(err)
	}
	gotResult0, err := dpfaggregator.MergePartialResult(result01, result02)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]dpfaggregator.CompleteHistogram{
		{Index: 0, Sum: 110},
		{Index: 1, Sum: 1100},
		{Index: 2, Sum: 1630},
		{Index: 3, Sum: 0},
	}, gotResult0, cmpopts.SortSlices(func(a, b dpfaggregator.CompleteHistogram) bool { return a.Index < b.Index })); diff != "" {
		t.Errorf("results mismatch (-want +got):\n%s", diff)
	}

	// Second-level aggregation: 5-bit prefixes.
	expandParamsURI1 := path.Join(expandParamsDir, "expand_params1")
	if err := cryptoio.SaveExpandParameters(ctx, &pb.ExpandParameters{
		Levels: []int32{4},
		Prefixes: &pb.HierarchicalPrefixes{
			Prefixes: []*pb.DomainPrefixes{
				{Prefix: []uint64{2, 3}},
			},
		},
		PreviousLevel: 1,
	}, expandParamsURI1); err != nil {
		t.Fatal(err)
	}

	partialHistogramURI11 := path.Join(partialResultDir1, "partial_histogram1")
	evaluationContextURI11 := path.Join(workspaceDir1, "evaluation_context1")
	if err := executeCommand(ctx, "dpf_aggregate_partial_report",
		"--partial_report_uri="+evaluationContextURI01,
		"--expand_parameters_uri="+expandParamsURI1,
		"--partial_histogram_uri="+partialHistogramURI11,
		"--evaluation_context_uri="+evaluationContextURI11,
		"--private_key_params_uri="+privateKeyURI,
	); err != nil {
		t.Fatal(err)
	}

	partialHistogramURI12 := path.Join(partialResultDir2, "partial_histogram1")
	evaluationContextURI12 := path.Join(workspaceDir2, "evaluation_context1")
	if err := executeCommand(ctx, "dpf_aggregate_partial_report",
		"--partial_report_uri="+evaluationContextURI02,
		"--expand_parameters_uri="+expandParamsURI1,
		"--partial_histogram_uri="+partialHistogramURI12,
		"--evaluation_context_uri="+evaluationContextURI12,
		"--private_key_params_uri="+privateKeyURI,
	); err != nil {
		t.Fatal(err)
	}

	result11, err := dpfaggregator.ReadPartialHistogram(ctx, partialHistogramURI11)
	if err != nil {
		t.Fatal(err)
	}
	result12, err := dpfaggregator.ReadPartialHistogram(ctx, partialHistogramURI12)
	if err != nil {
		t.Fatal(err)
	}
	gotResult1, err := dpfaggregator.MergePartialResult(result11, result12)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]dpfaggregator.CompleteHistogram{
		{Index: 16, Sum: 256},
		{Index: 17, Sum: 289},
		{Index: 18, Sum: 324},
		{Index: 19, Sum: 361},
		{Index: 20, Sum: 400},
		{Index: 21, Sum: 0},
		{Index: 22, Sum: 0},
		{Index: 23, Sum: 0},
		{Index: 24, Sum: 0},
		{Index: 25, Sum: 0},
		{Index: 26, Sum: 0},
		{Index: 27, Sum: 0},
		{Index: 28, Sum: 0},
		{Index: 29, Sum: 0},
		{Index: 30, Sum: 0},
		{Index: 31, Sum: 0},
	}, gotResult1, cmpopts.SortSlices(func(a, b dpfaggregator.CompleteHistogram) bool { return a.Index < b.Index })); diff != "" {
		t.Errorf("results mismatch (-want +got):\n%s", diff)
	}
}

func executeCommand(ctx context.Context, name string, args ...string) error {
	name = name+"_/"+name
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("err: %s; stderr: %s", err.Error(), stderr.String())
	}
	return nil
}