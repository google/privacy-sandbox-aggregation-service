// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reportutils

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/report/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/utils/utils"
)

func TestGetMaxKey(t *testing.T) {
	want := uint128.Max
	got := GetMaxBucketKey(128)
	if got.Cmp(want) != 0 {
		t.Fatalf("expect %s for key size %d, got %s", want.String(), 128, got.String())
	}

	var err error
	want, err = utils.StringToUint128("1180591620717411303423") // 2^70-1
	if err != nil {
		t.Fatal(err)
	}
	got = GetMaxBucketKey(70)
	if got.Cmp(want) != 0 {
		t.Fatalf("expect %s for key size %d, got %s", want.String(), 70, got.String())
	}
}

func TestParseRawReport(t *testing.T) {
	wantID, err := utils.StringToUint128("1180591620717411303423") // 2^70-1
	if err != nil {
		t.Fatal(err)
	}
	want := reporttypes.RawReport{
		Bucket: wantID,
		Value:  14,
	}
	got, err := ParseRawReport("1180591620717411303423,14", 70)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("incorrect result (-want +got):\n%s", diff)
	}

	wantErr := "key \"1180591620717411303423\" overflows the integer with 69 bits"
	_, err = ParseRawReport("1180591620717411303423,14", 69)
	if err == nil {
		t.Fatalf("expect error %q, got nil", wantErr)
	}

	gotErr := err.Error()
	if gotErr != wantErr {
		t.Fatalf("expect error %q, got %q", wantErr, gotErr)
	}
}

func TestReadRawReports(t *testing.T) {
	var want []reporttypes.RawReport
	for i := 5; i <= 20; i++ {
		for j := 0; j < i; j++ {
			want = append(want, reporttypes.RawReport{Bucket: uint128.From64(uint64(i)).Lsh(27), Value: uint64(i)})
		}
	}
	testFile, err := utils.RunfilesPath("report/test_raw_report_data.csv", false /*isBinary*/)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadRawReports(context.Background(), testFile, 32)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("wrong raw reports (-want +got):\n%s", diff)
	}
}
