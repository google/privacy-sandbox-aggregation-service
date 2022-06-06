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
// /path/to/dpf_merge_partial_aggregation \
// --partial_histogram_file1=/path/to/partial_histogram_file1.txt \
// --partial_histogram_file2=/path/to/partial_histogram_file2.txt \
// --complete_hisgogram_file=/path/to/complete_histogram_file.txt \
// --runner=direct
//
// 2. Dataflow on cloud
// /path/to/dpf_merge_partial_aggregation \
// --partial_histogram_file1=gs://<helper bucket>/partial_histogram_file1.txt \
// --partial_histogram_file2=gs://<helper bucket>/partial_histogram_file2.txt \
// --complete_histogram_file=gs://<reporter bucket>/complete_histogram_file.txt \
// --runner=dataflow \
// --project=<GCP project> \
// --temp_location=gs://<dataflow temp dir> \
// --staging_location=gs://<dataflow temp dir> \
// --worker_binary=/path/to/dpf_merge_partial_aggregation_pipeline

package main

import (
	"context"
	"flag"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/log"
	"github.com/apache/beam/sdks/go/pkg/beam/x/beamx"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"
)

var (
	partialHistogramURI1 = flag.String("partial_histogram_uri1", "", "Input partial histogram from helper 1.")
	partialHistogramURI2 = flag.String("partial_histogram_uri2", "", "Input partial histogram from helper 2.")
	completeHistogramURI = flag.String("complete_histogram_uri", "", "Output complete aggregation.")
)

func main() {
	flag.Parse()

	beam.Init()

	pipeline := beam.NewPipeline()
	scope := pipeline.Root()

	ctx := context.Background()

	inputExist, err := utils.IsFileGlobExist(ctx, *partialHistogramURI1)
	if err != nil {
		log.Exit(ctx, err)
	} else if !inputExist {
		log.Exitf(ctx, "input not found: %q", *partialHistogramURI1)
	}
	inputExist, err = utils.IsFileGlobExist(ctx, *partialHistogramURI2)
	if err != nil {
		log.Exit(ctx, err)
	} else if !inputExist {
		log.Exitf(ctx, "input not found: %q", *partialHistogramURI2)
	}

	dpfaggregator.MergePartialHistogram(scope, *partialHistogramURI1, *partialHistogramURI2, *completeHistogramURI)
	if err := beamx.Run(ctx, pipeline); err != nil {
		log.Exitf(ctx, "Failed to execute job: %s", err)
	}
}
