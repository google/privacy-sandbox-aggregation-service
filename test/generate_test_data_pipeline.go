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

// This binary reads the raw conversions from a file, and generates the encrypted reports that can be used as test data for the aggregation pipelines.
// If flag '-public_keys_uri2' is set, each report will be splitted into two shares for the MPC protocol;
// otherwise, each report simply gets encrypted for the one-party protocol.

// The pipeline can be executed in two ways:
//
// 1. Directly on local
// /path/to/generate_test_data_pipeline \
// --conversion_file=/path/to/conversion_data.csv \
// --partial_report_file1=/path/to/partial_report_1.txt \
// --partial_report_file2=/path/to/partial_report_2.txt \
// --public_key_file1=/path/to/public_key_file1 \
// --public_key_file2=/path/to/public_key_file2 \
// --runner=direct
//
// 2. Dataflow on cloud
// /path/to/generate_test_data_pipeline \
// --conversion_file=gs://<browser bucket>/conversion_data.csv \
// --partial_report_file1=gs://<helper bucket>/partial_report_1.txt \
// --partial_report_file2=gs://<helper bucket>/partial_report_2.txt \
// --public_key_file1=/path/to/public_key_file1 \
// --public_key_file2=/path/to/public_key_file2 \
// --runner=dataflow \
// --project=<GCP project> \
// --temp_location=gs://<dataflow temp dir> \
// --staging_location=gs://<dataflow temp dir> \
// --worker_binary=/path/to/generate_test_data_pipeline

package main

import (
	"context"
	"flag"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/log"
	"github.com/apache/beam/sdks/go/pkg/beam/x/beamx"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/test/dpfdataconverter"
	"github.com/google/privacy-sandbox-aggregation-service/test/onepartydataconverter"
)

var (
	conversionURI       = flag.String("conversion_uri", "", "Input raw conversion data.")
	encryptedReportURI1 = flag.String("encrypted_report_uri1", "", "Output encrypted report for helper 1.")
	encryptedReportURI2 = flag.String("encrypted_report_uri2", "", "Output encrypted report for helper 2.")

	publicKeysURI1 = flag.String("public_keys_uri1", "", "Input file containing the public keys from helper 1.")
	publicKeysURI2 = flag.String("public_keys_uri2", "", "Input file containing the public keys from helper 2.")
	keyBitSize     = flag.Int("key_bit_size", 32, "Bit size of the conversion keys.")

	encryptOutput = flag.Bool("encrypt_output", true, "Generate reports with encryption. This should only be false for integration test before HPKE is ready in Go Tink.")

	fileShards = flag.Int64("file_shards", 1, "The number of shards for the output file.")
)

func main() {
	flag.Parse()

	beam.Init()

	ctx := context.Background()
	var (
		helperPubKeys1, helperPubKeys2 *reporttypes.PublicKeys
		err                            error
	)
	helperPubKeys1, err = cryptoio.ReadPublicKeys(ctx, *publicKeysURI1)
	if err != nil {
		log.Exit(ctx, err)
	}
	if *publicKeysURI2 != "" {
		helperPubKeys2, err = cryptoio.ReadPublicKeys(ctx, *publicKeysURI2)
		if err != nil {
			log.Exit(ctx, err)
		}

	}

	pipeline := beam.NewPipeline()
	scope := pipeline.Root()

	if *publicKeysURI2 != "" {
		log.Infof(ctx, "encrypting the reports for MPC protocol")
		dpfdataconverter.GeneratePartialReport(scope, &dpfdataconverter.GeneratePartialReportParams{
			ConversionURI:     *conversionURI,
			PartialReportURI1: *encryptedReportURI1,
			PartialReportURI2: *encryptedReportURI2,
			KeyBitSize:        *keyBitSize,
			PublicKeys1:       helperPubKeys1,
			PublicKeys2:       helperPubKeys2,
			Shards:            *fileShards,
			EncryptOutput:     *encryptOutput,
		})
	} else {
		log.Infof(ctx, "encrypting the reports for one-party protocol")
		onepartydataconverter.GenerateEncryptedReport(scope, &onepartydataconverter.GenerateEncryptedReportParams{
			RawReportURI:       *conversionURI,
			EncryptedReportURI: *encryptedReportURI1,
			PublicKeys:         helperPubKeys1,
			Shards:             *fileShards,
			EncryptOutput:      *encryptOutput,
		})

	}

	if err := beamx.Run(ctx, pipeline); err != nil {
		log.Exitf(ctx, "Failed to execute job: %s", err)
	}
}
