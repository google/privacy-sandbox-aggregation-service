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

// This binary reads the raw conversions from a file, splits the conversions into secret DPF keys and encrypts the shares for different helpers.
// The pipeline can be executed in two ways:
//
// 1. Directly on local
// /path/to/dpf_generate_partial_report \
// --conversion_file=/path/to/conversion_data.csv \
// --partial_report_file1=/path/to/partial_report_1.txt \
// --partial_report_file2=/path/to/partial_report_2.txt \
// --public_key_file1=/path/to/public_key_file1 \
// --public_key_file2=/path/to/public_key_file2 \
// --runner=direct
//
// 2. Dataflow on cloud
// /path/to/dpf_generate_partial_report \
// --conversion_file=gs://<browser bucket>/conversion_data.csv \
// --partial_report_file1=gs://<helper bucket>/partial_report_1.txt \
// --partial_report_file2=gs://<helper bucket>/partial_report_2.txt \
// --public_key_file1=/path/to/public_key_file1 \
// --public_key_file2=/path/to/public_key_file2 \
// --runner=dataflow \
// --project=<GCP project> \
// --temp_location=gs://<dataflow temp dir> \
// --staging_location=gs://<dataflow temp dir> \
// --worker_binary=/path/to/dpf_generate_partial_report

package main

import (
	"context"
	"flag"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/log"
	"github.com/apache/beam/sdks/go/pkg/beam/x/beamx"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfbrowsersimulator"
)

var (
	conversionURI     = flag.String("conversion_uri", "", "Input raw conversion data.")
	partialReportURI1 = flag.String("partial_report_uri1", "", "Output partial report for helper 1.")
	partialReportURI2 = flag.String("partial_report_uri2", "", "Output partial report for helper 2.")

	publicKeysURI1 = flag.String("public_keys_uri1", "", "Input file containing the public keys from helper 1.")
	publicKeysURI2 = flag.String("public_keys_uri2", "", "Input file containing the public keys from helper 2.")
	keyBitSize     = flag.Int("key_bit_size", 32, "Bit size of the conversion keys.")

	fileShards = flag.Int64("file_shards", 1, "The number of shards for the output file.")
)

func main() {
	flag.Parse()

	beam.Init()

	ctx := context.Background()
	helperPubKeys1, err := cryptoio.ReadPublicKeyVersions(ctx, *publicKeysURI1)
	if err != nil {
		log.Exit(ctx, err)
	}
	helperPubKeys2, err := cryptoio.ReadPublicKeyVersions(ctx, *publicKeysURI2)
	if err != nil {
		log.Exit(ctx, err)
	}

	// Use any version of the public keys until the version control is designed.
	var publicKeyInfo1, publicKeyInfo2 []cryptoio.PublicKeyInfo
	for _, v := range helperPubKeys1 {
		publicKeyInfo1 = v
	}
	for _, v := range helperPubKeys2 {
		publicKeyInfo2 = v
	}

	pipeline := beam.NewPipeline()
	scope := pipeline.Root()

	dpfbrowsersimulator.GeneratePartialReport(scope, &dpfbrowsersimulator.GeneratePartialReportParams{
		ConversionURI:     *conversionURI,
		PartialReportURI1: *partialReportURI1,
		PartialReportURI2: *partialReportURI2,
		KeyBitSize:        int32(*keyBitSize),
		PublicKeys1:       publicKeyInfo1,
		PublicKeys2:       publicKeyInfo2,
		Shards:            *fileShards,
	})
	if err := beamx.Run(ctx, pipeline); err != nil {
		log.Exitf(ctx, "Failed to execute job: %s", err)
	}
}
