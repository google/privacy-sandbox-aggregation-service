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

// This binary aggregates the partial report for each aggregation ID, which is calculated from the exponentiated conversion keys from the other helper.
// The pipeline can be executed in two ways:
//
// 1. Directly on local
// /path/to/aggregate_partial_report \
// --partial_report_file=/path/to/partial_report_file.txt \
// --reencrypted_key_file=/path/to/reencrypted_key_file.txt \
// --partial_aggregation_file=/path/to/partial_aggregation_file.txt \
// --private_key_dir=/path/to/private_key_dir \
// --runner=direct
//
// 2. Dataflow on cloud
// /path/to/aggregate_partial_report \
// --partial_report_file=gs://<helper bucket>/partial_report_file.txt \
// --reencrypted_key_file=gs://<helper bucket>/reencrypted_key_file.txt \
// --partial_aggregation_file=gs://<helper bucket>/partial_aggregation_file.txt \
// --private_key_dir=/path/to/private_key_dir \
// --runner=dataflow \
// --project=<GCP project> \
// --temp_location=gs://<dataflow temp dir> \
// --staging_location=gs://<dataflow temp dir> \
// --worker_binary=/path/to/aggregate_partial_report
package main

import (
	"context"
	"flag"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/log"
	"github.com/apache/beam/sdks/go/pkg/beam/x/beamx"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/conversion"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/conversionaggregator"
)

var (
	partialReportFile      = flag.String("partial_report_file", "", "Input partial reports.")
	reencryptedKeyFile     = flag.String("reencrypted_key_file", "", "Input reencrypted conversion keys.")
	partialAggregationFile = flag.String("partial_aggregation_file", "", "Output partial aggregation.")
	ignorePrivacy          = flag.Bool("ignore_privacy", false, "Whether to ignore privacy during aggregation.")
	privateKeyDir          = flag.String("private_key_dir", "", "Directory for private keys and exponential secret.")
	privacyEpsilon         = flag.Float64("privacy_epsilon", 6, "Epsilon for Laplace noise.")
	privacyDelta           = flag.Float64("privacy_delta", 1e-5, "Delta for Laplace noise.")
	privacyMinValue        = flag.Float64("privacy_min_value", 0, "Minimum value to include in the summation.")
	privacyMaxValue        = flag.Float64("privacy_max_value", 100, "Maximum value to include in the summation.")
)

func main() {
	flag.Parse()
	beam.Init()
	ctx := context.Background()
	helperInfo, err := conversion.GetPrivateInfo(*privateKeyDir)
	if err != nil {
		log.Exit(ctx, err)
	}
	pipeline := beam.NewPipeline()
	scope := pipeline.Root()
	conversionaggregator.AggregatePartialReport(
		scope,
		&conversionaggregator.AggregateParams{
			PartialReport:      *partialReportFile,
			ReencryptedKey:     *reencryptedKeyFile,
			PartialAggregation: *partialAggregationFile,
			HelperInfo:         helperInfo,
			IgnorePrivacy:      *ignorePrivacy,
		},
		conversionaggregator.PrivacyParams{
			Epsilon:  *privacyEpsilon,
			Delta:    *privacyDelta,
			MinValue: *privacyMinValue,
			MaxValue: *privacyMaxValue,
		})
	if err := beamx.Run(ctx, pipeline); err != nil {
		log.Exitf(ctx, "Failed to execute job: %s", err)
	}
}
