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

// This binary merges the partial aggregations from two helpers and calculates the complete aggregation.
// The pipeline can be executed in two ways:
//
// 1. Directly on local
// /path/to/merge_partial_aggregation \
// --partial_aggregation_file1=/path/to/partial_aggregation_file1.txt \
// --partial_aggregation_file2=/path/to/partial_aggregation_file2.txt \
// --complete_aggregation_file=/path/to/complete_aggregation_file.txt \
// --runner=direct
//
// 2. Dataflow on cloud
// /path/to/merge_partial_aggregation \
// --partial_aggregation_file1=gs://<helper bucket>/partial_aggregation_file1.txt \
// --partial_aggregation_file2=gs://<helper bucket>/partial_aggregation_file2.txt \
// --complete_aggregation_file=gs://<reporter bucket>/complete_aggregation_file.txt \
// --runner=dataflow \
// --project=<GCP project> \
// --temp_location=gs://<dataflow temp dir> \
// --staging_location=gs://<dataflow temp dir> \
// --worker_binary=/path/to/merge_partial_aggregation

package main

import (
	"context"
	"flag"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/log"
	"github.com/apache/beam/sdks/go/pkg/beam/x/beamx"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/conversionaggregator"
)

var (
	partialAggregationFile1 = flag.String("partial_aggregation_file1", "", "Input partial aggregation from helper 1.")
	partialAggregationFile2 = flag.String("partial_aggregation_file2", "", "Input partial aggregation from helper 2.")
	completeAggregationFile = flag.String("complete_aggregation_file", "", "Output complete aggregation.")
)

func main() {
	flag.Parse()

	beam.Init()

	pipeline := beam.NewPipeline()
	scope := pipeline.Root()

	ctx := context.Background()
	conversionaggregator.MergePartialAggregation(scope, *partialAggregationFile1, *partialAggregationFile2, *completeAggregationFile)
	if err := beamx.Run(ctx, pipeline); err != nil {
		log.Exitf(ctx, "Failed to execute job: %s", err)
	}
}
