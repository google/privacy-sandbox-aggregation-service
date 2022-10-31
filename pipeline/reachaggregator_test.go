package reachaggregator

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"lukechampine.com/uint128"
)

func TestWriteReadReachResults(t *testing.T) {
	fileDir, err := ioutil.TempDir("/tmp", "test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(fileDir)

	want := map[uint128.Uint128]*ReachResult{
		uint128.From64(0): &ReachResult{Verification: 1, Count: 2},
		uint128.From64(3): &ReachResult{Verification: 4, Count: 5},
	}

	filePath := filepath.Join(fileDir, "reach_results")
	ctx := context.Background()
	err = WriteReachResult(ctx, want, filePath)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ReadReachResult(ctx, filePath)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("results mismatch (-want +got):\n%s", diff)
	}
}
