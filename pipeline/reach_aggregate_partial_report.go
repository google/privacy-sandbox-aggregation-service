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
// /path/to/dpf_aggregate_partial_report \
// --partial_report_file=/path/to/partial_report_file.txt \
// --partial_histogram_file=/path/to/partial_histogram_file.txt \
// --private_key_dir=/path/to/private_key_dir \
// --runner=direct
//
// 2. Dataflow on cloud
// /path/to/dpf_aggregate_partial_report \
// --partial_report_file=gs://<helper bucket>/partial_report_file.txt \
// --partial_histogram_file=gs://<helper bucket>/partial_histogram_file.txt \
// --private_key_dir=/path/to/private_key_dir \
// --runner=dataflow \
// --project=<GCP project> \
// --temp_location=gs://<dataflow temp dir> \
// --staging_location=gs://<dataflow temp dir> \
// --worker_binary=/path/to/dpf_aggregate_partial_report
package main

import (
	"context"
	"flag"
	"os"
	"runtime/pprof"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/log"
	"github.com/apache/beam/sdks/go/pkg/beam/x/beamx"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/reachaggregator"
)

var (
	partialReportFile    = flag.String("partial_report_file", "", "Input partial reports.")
	partialHistogramFile = flag.String("partial_histogram_file", "", "Output partial aggregation.")
	keyBitSize           = flag.Int("key_bit_size", 32, "Bit size of the conversion keys.")
	privateKeyParamsFile = flag.String("private_key_params_file", "", "Input file that stores the parameters required to read the standard private keys.")

	directCombine = flag.Bool("direct_combine", true, "Use direct or segmented combine when aggregating the expanded vectors.")
	segmentLength = flag.Uint64("segment_length", 32768, "Segment length to split the original vectors.")

	epsilon       = flag.Float64("epsilon", 0.0, "Epsilon for the privacy budget.")
	l1Sensitivity = flag.Uint64("l1_sensitivity", 1, "L1-sensitivity for the privacy budget.")

	fileShards = flag.Int64("file_shards", 1, "The number of shards for the output file.")

	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(ctx, err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	beam.Init()

	helperPrivKeys, err := cryptoio.ReadPrivateKeyCollection(ctx, *privateKeyParamsFile)
	if err != nil {
		log.Exit(ctx, err)
	}

	pipeline := beam.NewPipeline()
	scope := pipeline.Root()
	if err := reachaggregator.AggregatePartialReport(
		scope,
		&reachaggregator.AggregatePartialReportParams{
			PartialReportFile:    *partialReportFile,
			PartialHistogramFile: *partialHistogramFile,
			HelperPrivateKeys:    helperPrivKeys,
			DirectCombine:        *directCombine,
			SegmentLength:        *segmentLength,
			Shards:               *fileShards,
			KeyBitSize:           int32(*keyBitSize),
			Epsilon:              *epsilon,
			L1Sensitivity:        *l1Sensitivity,
		}); err != nil {
		log.Exit(ctx, err)
	}
	if err := beamx.Run(ctx, pipeline); err != nil {
		log.Exitf(ctx, "Failed to execute job: %s", err)
	}
}
