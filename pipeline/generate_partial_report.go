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

// This binary reads the raw conversions from a file, splits the conversions into secret shares and encrypts the shares for different helpers.
// The pipeline can be executed in two ways:
//
// 1. Directly on local
// /path/to/generate_partial_report \
// --conversion_file=/path/to/conversion_data.csv \
// --partial_report_file1=/path/to/partial_report_1.txt \
// --partial_report_file2=/path/to/partial_report_2.txt \
// --public_key_dir1=/path/to/public_key_dir1 \
// --public_key_dir2=/path/to/public_key_dir2 \
// --runner=direct
//
// 2. Dataflow on cloud
// /path/to/generate_partial_report \
// --conversion_file=gs://<browser bucket>/conversion_data.csv \
// --partial_report_file1=gs://<helper bucket>/partial_report_1.txt \
// --partial_report_file2=gs://<helper bucket>/partial_report_2.txt \
// --public_key_dir1=/path/to/public_key_dir1 \
// --public_key_dir2=/path/to/public_key_dir2 \
// --runner=dataflow \
// --project=<GCP project> \
// --temp_location=gs://<dataflow temp dir> \
// --staging_location=gs://<dataflow temp dir> \
// --worker_binary=/path/to/generate_partial_report

package main

import (
	"context"
	"flag"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/log"
	"github.com/apache/beam/sdks/go/pkg/beam/x/beamx"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/browsersimulator"
)

var (
	conversionFile     = flag.String("conversion_file", "", "Input raw conversion data.")
	partialReportFile1 = flag.String("partial_report_file1", "", "Output partial report for helper 1.")
	partialReportFile2 = flag.String("partial_report_file2", "", "Output partial report for helper 2.")

	publicKeyDir1 = flag.String("public_key_dir1", "", "Directory for public keys from helper 1.")
	publicKeyDir2 = flag.String("public_key_dir2", "", "Directory for public keys from helper 2.")

	fileShards = flag.Int64("file_shards", 1, "The number of shards for the output file.")
)

func main() {
	flag.Parse()

	beam.Init()

	ctx := context.Background()
	helperInfo1, err := browsersimulator.GetPublicInfo(*publicKeyDir1)
	if err != nil {
		log.Exit(ctx, err)
	}
	helperInfo2, err := browsersimulator.GetPublicInfo(*publicKeyDir2)
	if err != nil {
		log.Exit(ctx, err)
	}

	pipeline := beam.NewPipeline()
	scope := pipeline.Root()

	browsersimulator.GeneratePartialReport(scope, *conversionFile, *partialReportFile1, *partialReportFile2, helperInfo1, helperInfo2, *fileShards)
	if err := beamx.Run(ctx, pipeline); err != nil {
		log.Exitf(ctx, "Failed to execute job: %s", err)
	}
}
