package dpf_pipeline_integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/utils/utils"
)

const keyBitSize = 32

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestPipeline(t *testing.T) {
	testPipeline(t, false /*encryptOutput*/)
	testPipeline(t, true /*encryptOutput*/)
}

func testPipeline(t testing.TB, encryptOutput bool) {
	ctx := context.Background()

	testFile, err := utils.RunfilesPath("report/test_raw_report_data.csv", false /*isBinary*/)
	if err != nil {
		t.Fatal(err)
	}
	createKeyBinary, err := utils.RunfilesPath("tools/create_hybrid_key_pair", true /*isBinary*/)
	if err != nil {
		t.Fatal(err)
	}
	generateTestDataBinary, err := utils.RunfilesPath("internal/generate_test_data_pipeline", true /*isBinary*/)
	if err != nil {
		t.Fatal(err)
	}
	dpfAggregateBinary, err := utils.RunfilesPath("pipeline/dpf_aggregate_partial_report_pipeline", true /*isBinary*/)
	if err != nil {
		t.Fatal(err)
	}

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
	if err := executeCommand(ctx, createKeyBinary,
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
	if err := executeCommand(ctx, generateTestDataBinary,
		"--conversion_uri="+testFile,
		"--encrypted_report_uri1="+partialReportURI1,
		"--encrypted_report_uri2="+partialReportURI2,
		"--public_keys_uri1="+publicKeyURI,
		"--public_keys_uri2="+publicKeyURI,
		"--key_bit_size="+strconv.Itoa(keyBitSize),
		"--encrypt_output="+strconv.FormatBool(encryptOutput),
	); err != nil {
		t.Fatal(err)
	}

	// First-level aggregation: 2-bit prefixes.
	expandParamsDir := path.Join(tmpDir, "expand_params_dir")
	if err := os.MkdirAll(expandParamsDir, 0755); err != nil {
		t.Fatal(err)
	}
	expandParamsURI0 := path.Join(expandParamsDir, "expand_params0")
	if err := dpfaggregator.SaveExpandParameters(ctx, &dpfaggregator.ExpandParameters{
		Levels:        []int32{1},
		Prefixes:      [][]uint128.Uint128{{}},
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
	decryptedReportURI1 := path.Join(workspaceDir1, "decrypted_report")
	if err := executeCommand(ctx, dpfAggregateBinary,
		"--partial_report_uri="+partialReportURI1,
		"--expand_parameters_uri="+expandParamsURI0,
		"--partial_histogram_uri="+partialHistogramURI01,
		"--decrypted_report_uri="+decryptedReportURI1,
		"--private_key_params_uri="+privateKeyURI,
		"--key_bit_size="+strconv.Itoa(keyBitSize),
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
	decryptedReportURI2 := path.Join(workspaceDir2, "decrypted_report")
	if err := executeCommand(ctx, dpfAggregateBinary,
		"--partial_report_uri="+partialReportURI2,
		"--expand_parameters_uri="+expandParamsURI0,
		"--partial_histogram_uri="+partialHistogramURI02,
		"--decrypted_report_uri="+decryptedReportURI2,
		"--private_key_params_uri="+privateKeyURI,
		"--key_bit_size="+strconv.Itoa(keyBitSize),
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
		{Bucket: uint128.From64(0), Sum: 110},
		{Bucket: uint128.From64(1), Sum: 1100},
		{Bucket: uint128.From64(2), Sum: 1630},
		{Bucket: uint128.From64(3), Sum: 0},
	}, gotResult0, cmpopts.SortSlices(func(a, b dpfaggregator.CompleteHistogram) bool { return a.Bucket.Cmp(b.Bucket) == -1 })); diff != "" {
		t.Errorf("results mismatch (-want +got):\n%s", diff)
	}

	// Second-level aggregation: 5-bit prefixes.
	expandParamsURI1 := path.Join(expandParamsDir, "expand_params1")
	if err := dpfaggregator.SaveExpandParameters(ctx, &dpfaggregator.ExpandParameters{
		Levels:        []int32{4},
		Prefixes:      [][]uint128.Uint128{{uint128.From64(2), uint128.From64(3)}},
		PreviousLevel: 1,
	}, expandParamsURI1); err != nil {
		t.Fatal(err)
	}

	partialHistogramURI11 := path.Join(partialResultDir1, "partial_histogram1")
	if err := executeCommand(ctx, dpfAggregateBinary,
		"--partial_report_uri="+decryptedReportURI1,
		"--expand_parameters_uri="+expandParamsURI1,
		"--partial_histogram_uri="+partialHistogramURI11,
		"--key_bit_size="+strconv.Itoa(keyBitSize),
	); err != nil {
		t.Fatal(err)
	}

	partialHistogramURI12 := path.Join(partialResultDir2, "partial_histogram1")
	if err := executeCommand(ctx, dpfAggregateBinary,
		"--partial_report_uri="+decryptedReportURI2,
		"--expand_parameters_uri="+expandParamsURI1,
		"--partial_histogram_uri="+partialHistogramURI12,
		"--key_bit_size="+strconv.Itoa(keyBitSize),
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
		{Bucket: uint128.From64(16), Sum: 256},
		{Bucket: uint128.From64(17), Sum: 289},
		{Bucket: uint128.From64(18), Sum: 324},
		{Bucket: uint128.From64(19), Sum: 361},
		{Bucket: uint128.From64(20), Sum: 400},
		{Bucket: uint128.From64(21), Sum: 0},
		{Bucket: uint128.From64(22), Sum: 0},
		{Bucket: uint128.From64(23), Sum: 0},
		{Bucket: uint128.From64(24), Sum: 0},
		{Bucket: uint128.From64(25), Sum: 0},
		{Bucket: uint128.From64(26), Sum: 0},
		{Bucket: uint128.From64(27), Sum: 0},
		{Bucket: uint128.From64(28), Sum: 0},
		{Bucket: uint128.From64(29), Sum: 0},
		{Bucket: uint128.From64(30), Sum: 0},
		{Bucket: uint128.From64(31), Sum: 0},
	}, gotResult1, cmpopts.SortSlices(func(a, b dpfaggregator.CompleteHistogram) bool { return a.Bucket.Cmp(b.Bucket) == -1 })); diff != "" {
		t.Errorf("results mismatch (-want +got):\n%s", diff)
	}
}

func executeCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("err: %s; stderr: %s", err.Error(), stderr.String())
	}
	return nil
}
