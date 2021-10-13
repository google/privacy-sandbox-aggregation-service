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

// Package reportutils contains util functions for processing the raw reports.
package reportutils

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/report/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/utils/utils"
)

// GetMaxBucketKey calculates the maximum bucket ID for a given length of bits.
func GetMaxBucketKey(s int) uint128.Uint128 {
	maxKey := uint128.Max
	if s < 128 {
		maxKey = uint128.Uint128{1, 0}.Lsh(uint(s)).Sub64(1)
	}
	return maxKey
}

//ParseRawReport parses a raw conversion.
func ParseRawReport(line string, keyBitSize int) (reporttypes.RawReport, error) {
	cols := strings.Split(line, ",")
	if got, want := len(cols), 2; got != want {
		return reporttypes.RawReport{}, fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
	}

	key128, err := utils.StringToUint128(cols[0])
	if err != nil {
		return reporttypes.RawReport{}, err
	}
	if key128.Cmp(GetMaxBucketKey(keyBitSize)) == 1 {
		return reporttypes.RawReport{}, fmt.Errorf("key %q overflows the integer with %d bits", key128.String(), keyBitSize)
	}

	value64, err := strconv.ParseUint(cols[1], 10, 64)
	if err != nil {
		return reporttypes.RawReport{}, err
	}
	return reporttypes.RawReport{Bucket: key128, Value: value64}, nil
}

// ReadRawReports reads conversions from a file. Each line of the file represents a conversion record.
func ReadRawReports(ctx context.Context, conversionFile string, keyBitSize int) ([]reporttypes.RawReport, error) {
	lines, err := utils.ReadLines(ctx, conversionFile)
	if err != nil {
		return nil, err
	}

	var conversions []reporttypes.RawReport
	for _, l := range lines {
		conversion, err := ParseRawReport(l, keyBitSize)
		if err != nil {
			return nil, err
		}
		conversions = append(conversions, conversion)
	}
	return conversions, nil
}
