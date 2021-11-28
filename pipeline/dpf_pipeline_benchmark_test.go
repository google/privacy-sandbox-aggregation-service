package dpf_pipeline_benchmark_test

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

	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
)

const keyBitSize = 32

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func BenchmarkPipeline(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testDirectPipeline(b, true /*withEncryption*/)
	}
}

func generateLargePrefixes(count int) []uint128.Uint128 {
	output := make([]uint128.Uint128, count)
	for i := 0; i < count; i++ {
		output[i] = uint128.From64(uint64(i))
	}
	return output
}

func testDirectPipeline(t testing.TB, encryptOutput bool) {
	ctx := context.Background()

	testFile, err := ioutils.RunfilesPath("pipeline/dpf_test_conversion_data.csv", false /*isBinary*/)
	if err != nil {
		t.Fatal(err)
	}
	createKeyBinary, err := ioutils.RunfilesPath("pipeline/create_hybrid_key_pair", true /*isBinary*/)
	if err != nil {
		t.Fatal(err)
	}
	generateTestDataBinary, err := ioutils.RunfilesPath("pipeline/generate_test_data_pipeline", true /*isBinary*/)
	if err != nil {
		t.Fatal(err)
	}
	dpfAggregateBinary, err := ioutils.RunfilesPath("pipeline/dpf_aggregate_partial_report_pipeline", true /*isBinary*/)
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

	expandParamsDir := path.Join(tmpDir, "expand_params_dir")
	if err := os.MkdirAll(expandParamsDir, 0755); err != nil {
		t.Fatal(err)
	}
	expandParamsURI := path.Join(expandParamsDir, "expand_params")
	if err := dpfaggregator.SaveExpandParameters(ctx, &dpfaggregator.ExpandParameters{
		Levels:        []int32{4},
		Prefixes:      [][]uint128.Uint128{generateLargePrefixes(1048576)},
		PreviousLevel: -1,
	}, expandParamsURI); err != nil {
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
	partialHistogramURI1 := path.Join(partialResultDir1, "partial_histogram")
	decryptedReportURI1 := path.Join(workspaceDir1, "decrypted_report")
	if err := executeCommand(ctx, dpfAggregateBinary,
		"--partial_report_uri="+partialReportURI1,
		"--expand_parameters_uri="+expandParamsURI,
		"--partial_histogram_uri="+partialHistogramURI1,
		"--decrypted_report_uri="+decryptedReportURI1,
		"--private_key_params_uri="+privateKeyURI,
		"--key_bit_size="+strconv.Itoa(keyBitSize),
		"--hierarchical_expand=false",
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
	partialHistogramURI2 := path.Join(partialResultDir2, "partial_histogram")
	decryptedReportURI2 := path.Join(workspaceDir2, "decrypted_report")
	if err := executeCommand(ctx, dpfAggregateBinary,
		"--partial_report_uri="+partialReportURI2,
		"--expand_parameters_uri="+expandParamsURI,
		"--partial_histogram_uri="+partialHistogramURI2,
		"--decrypted_report_uri="+decryptedReportURI2,
		"--private_key_params_uri="+privateKeyURI,
		"--key_bit_size="+strconv.Itoa(keyBitSize),
		"--hierarchical_expand=false",
	); err != nil {
		t.Fatal(err)
	}

	// 	result1, err := dpfaggregator.ReadPartialHistogram(ctx, partialHistogramURI1)
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}
	// 	result2, err := dpfaggregator.ReadPartialHistogram(ctx, partialHistogramURI2)
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}
	// 	gotResult, err := dpfaggregator.MergePartialResult(result1, result2)
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}

	// 	if diff := cmp.Diff([]dpfaggregator.CompleteHistogram{
	// 		{Bucket: uint128.From64(19), Sum: 361},
	// 		{Bucket: uint128.From64(20), Sum: 400},
	// 		{Bucket: uint128.From64(21), Sum: 0},
	// 	}, gotResult, cmpopts.SortSlices(func(a, b dpfaggregator.CompleteHistogram) bool { return a.Bucket.Cmp(b.Bucket) == -1 })); diff != "" {
	// 		t.Errorf("results mismatch (-want +got):\n%s", diff)
	// 	}
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
