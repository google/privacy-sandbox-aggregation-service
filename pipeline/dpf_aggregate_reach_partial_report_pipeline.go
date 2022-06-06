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

// This binary aggregates the partial report for the Reach frequency.
package main

import (
	"context"
	"flag"
	"math"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/log"
	"github.com/apache/beam/sdks/go/pkg/beam/x/beamx"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelineutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/reachaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"
)

var (
	partialReportURI    = flag.String("partial_report_uri", "", "Input partial reports. It may contain the original encrypted partial reports or evaluation context.")
	partialHistogramURI = flag.String("partial_histogram_uri", "", "Output location of partial aggregation.")
	partialValidityURI  = flag.String("partial_validity_uri", "", "Output location of partial validity.")
	keyBitSize          = flag.Int("key_bit_size", 32, "Bit size of the data bucket keys. Support up to 128 bit.")
	privateKeyParamsURI = flag.String("private_key_params_uri", "", "Input file that stores the parameters required to read the standard private keys.")

	directCombine = flag.Bool("direct_combine", false, "Use direct or segmented combine when aggregating the expanded vectors.")
	segmentLength = flag.Uint64("segment_length", 32768, "Segment length to split the original vectors.")

	epsilon = flag.Float64("epsilon", 0.0, "Epsilon for the privacy budget.")
	// The default l1 sensitivity is consistent with:
	// https://github.com/WICG/conversion-measurement-api/blob/main/AGGREGATE.md#privacy-budgeting
	l1Sensitivity = flag.Uint64("l1_sensitivity", uint64(math.Pow(2, 16)), "L1-sensitivity for the privacy budget.")

	fileShards = flag.Int64("file_shards", 10, "The number of shards for the output file.")
)

func main() {
	flag.Parse()
	beam.Init()

	ctx := context.Background()
	helperPrivKeys, err := cryptoio.ReadPrivateKeyCollection(ctx, *privateKeyParamsURI)
	if err != nil {
		log.Exit(ctx, err)
	}

	log.Infof(ctx, "Output data written to %v file shards", *fileShards)

	inputGlob := pipelineutils.AddStrInPath(*partialReportURI, "*")
	inputExist, err := utils.IsFileGlobExist(ctx, inputGlob)
	if err != nil {
		log.Exit(ctx, err)
	} else if !inputExist {
		log.Exitf(ctx, "input not found: %q", inputGlob)
	}

	pipeline := beam.NewPipeline()
	scope := pipeline.Root()

	if err := reachaggregator.AggregatePartialReport(
		scope,
		&reachaggregator.AggregatePartialReportParams{
			PartialReportURI:    *partialReportURI,
			PartialHistogramURI: *partialHistogramURI,
			PartialValidityURI:  *partialValidityURI,
			HelperPrivateKeys:   helperPrivKeys,
			KeyBitSize:          *keyBitSize,
			CombineParams: &dpfaggregator.CombineParams{
				DirectCombine: *directCombine,
				SegmentLength: *segmentLength,
				Epsilon:       *epsilon,
				L1Sensitivity: *l1Sensitivity,
			},
			Shards: *fileShards,
		}); err != nil {
		log.Exit(ctx, err)
	}
	if err := beamx.Run(ctx, pipeline); err != nil {
		log.Exitf(ctx, "Failed to execute job: %s", err)
	}
}
