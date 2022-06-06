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

// This binary decrypts the encrypted reports generated for the one-party service design and aggregates them.
package main

import (
	"context"
	"flag"
	"math"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/log"
	"github.com/apache/beam/sdks/go/pkg/beam/x/beamx"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/onepartyaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelineutils"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"
)

var (
	encryptedReportURI  = flag.String("encrypted_report_uri", "", "Input encrypted reports.")
	targetBucketURI     = flag.String("target_bucket_uri", "", "Input target buckets.")
	histogramURI        = flag.String("histogram_uri", "", "Output aggregation results.")
	privateKeyParamsURI = flag.String("private_key_params_uri", "", "Input file that stores the parameters required to read the standard private keys.")
	epsilon             = flag.Float64("epsilon", 0.0, "Epsilon for the privacy budget.")
	// The default l1 sensitivity is consistent with:
	// https://github.com/WICG/conversion-measurement-api/blob/main/AGGREGATE.md#privacy-budgeting
	l1Sensitivity = flag.Uint64("l1_sensitivity", uint64(math.Pow(2, 16)), "L1-sensitivity for the privacy budget.")
)

func main() {
	flag.Parse()
	beam.Init()

	ctx := context.Background()
	helperPrivKeys, err := cryptoio.ReadPrivateKeyCollection(ctx, *privateKeyParamsURI)
	if err != nil {
		log.Exit(ctx, err)
	}

	inputGlob := pipelineutils.AddStrInPath(*encryptedReportURI, "*")
	inputExist, err := utils.IsFileGlobExist(ctx, inputGlob)
	if err != nil {
		log.Exit(ctx, err)
	} else if !inputExist {
		log.Exitf(ctx, "input not found: %q", inputGlob)
	}

	pipeline := beam.NewPipeline()
	scope := pipeline.Root()
	onepartyaggregator.AggregateReport(
		scope,
		&onepartyaggregator.AggregateReportParams{
			EncryptedReportURI: *encryptedReportURI,
			TargetBucketURI:    *targetBucketURI,
			HistogramURI:       *histogramURI,
			HelperPrivateKeys:  helperPrivKeys,
			Epsilon:            *epsilon,
			L1Sensitivity:      *l1Sensitivity,
		})

	if err := beamx.Run(ctx, pipeline); err != nil {
		log.Exitf(ctx, "Failed to execute job: %s", err)
	}
}
