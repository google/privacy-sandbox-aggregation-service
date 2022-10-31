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

// This binary reads the raw Reach reports from a file, and generates the encrypted reports that can be used as test data for the aggregation pipelines.
package main

import (
	"context"
	"flag"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/log"
	"github.com/apache/beam/sdks/go/pkg/beam/x/beamx"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/test/reachdataconverter"
)

var (
	conversionURI       = flag.String("conversion_uri", "", "Input raw conversion data.")
	encryptedReportURI1 = flag.String("encrypted_report_uri1", "", "Output encrypted report for helper 1.")
	encryptedReportURI2 = flag.String("encrypted_report_uri2", "", "Output encrypted report for helper 2.")

	publicKeysURI1 = flag.String("public_keys_uri1", "", "Input file containing the public keys from helper 1.")
	publicKeysURI2 = flag.String("public_keys_uri2", "", "Input file containing the public keys from helper 2.")
	keyBitSize     = flag.Int("key_bit_size", 32, "Bit size of the conversion keys.")

	useHierarchy  = flag.Bool("use_hierarchy", false, "Use hierarchies when creating DPF keys.")
	fullHierarchy = flag.Bool("full_hierarchy", false, "Use every bit in the domain as a hierarchy.")
	prefixBitSize = flag.Int("prefix_bit_size", 0, "Bit size of the zero prefixes.")
	evalBitSize   = flag.Int("eval_bit_size", 0, "Bit size to evaluate.")

	fileShards = flag.Int64("file_shards", 1, "The number of shards for the output file.")
)

func main() {
	flag.Parse()

	beam.Init()

	ctx := context.Background()
	helperPubKeys1, err := cryptoio.ReadPublicKeys(ctx, *publicKeysURI1)
	if err != nil {
		log.Exit(ctx, err)
	}
	helperPubKeys2, err := cryptoio.ReadPublicKeys(ctx, *publicKeysURI2)
	if err != nil {
		log.Exit(ctx, err)
	}

	pipeline := beam.NewPipeline()
	scope := pipeline.Root()

	reachdataconverter.GeneratePartialReport(scope, &reachdataconverter.GeneratePartialReportParams{
		ReachReportURI:    *conversionURI,
		PartialReportURI1: *encryptedReportURI1,
		PartialReportURI2: *encryptedReportURI2,
		PublicKeys1:       helperPubKeys1,
		PublicKeys2:       helperPubKeys2,
		KeyBitSize:        *keyBitSize,
		UseHierarchy:      *useHierarchy,
		FullHierarchy:     *fullHierarchy,
		PrefixBitSize:     *prefixBitSize,
		EvalBitSize:       *evalBitSize,
		Shards:            *fileShards,
	})

	if err := beamx.Run(ctx, pipeline); err != nil {
		log.Exitf(ctx, "Failed to execute job: %s", err)
	}
}
