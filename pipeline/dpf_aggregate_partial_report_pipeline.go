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

// This binary aggregates the partial report for the bucket ID(or prefixes) specified by the expand parameters.
// The pipeline can be executed in two ways:
//
// 1. Directly on local with flag '--runner=direct'
//
// 2. Dataflow on cloud with flag '--runner=dataflow', and the following flags need to be set:
// --project=<GCP project>
// --region=<worker region>
// --temp_location=gs://<dataflow temp dir>
// --staging_location=gs://<dataflow temp dir>
// --worker_binary=/path/to/dpf_aggregate_partial_report_pipeline/binary
// --zone=<worker zone> (optional)
// --max_num_workers=<number> (optional)
// --worker_machine_type=<GCE instance type> (optional)
// --job_name=<unique ongoing job name> (optional)
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
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

var (
	partialReportURI    = flag.String("partial_report_uri", "", "Input partial reports. It may contain the original encrypted partial reports or evaluation context.")
	expandParametersURI = flag.String("expand_parameters_uri", "", "Input URI of the expansion parameter file.")
	partialHistogramURI = flag.String("partial_histogram_uri", "", "Output location of partial aggregation.")
	decryptedReportURI  = flag.String("decrypted_report_uri", "", "Output location of the decrypted partial reports for hierarchical query so the helper won't need to do the decryption repeatedly.")
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
	expandParams, err := dpfaggregator.ReadExpandParameters(ctx, *expandParametersURI)
	if err != nil {
		log.Exit(ctx, err)
	}

	inputGlob := pipelineutils.AddStrInPath(*partialReportURI, "*")
	inputExist, err := utils.IsFileGlobExist(ctx, inputGlob)
	if err != nil {
		log.Exit(ctx, err)
	} else if !inputExist {
		log.Exitf(ctx, "input not found: %q", inputGlob)
	}

	var helperPrivKeys map[string]*pb.StandardPrivateKey
	// Private keys are only needed when aggregating the partial reports for the first time.
	// Otherwise partialReportURI should point to the decrypted reports.
	if expandParams.PreviousLevel == -1 {
		helperPrivKeys, err = cryptoio.ReadPrivateKeyCollection(ctx, *privateKeyParamsURI)
		if err != nil {
			log.Exit(ctx, err)
		}
		if *decryptedReportURI == "" && !expandParams.DirectExpansion {
			log.Exitf(ctx, "expect non-empty output decrypt report URI")
		}
	}

	log.Infof(ctx, "Output data written to %v file shards", *fileShards)

	pipeline := beam.NewPipeline()
	scope := pipeline.Root()
	if err := dpfaggregator.AggregatePartialReport(
		scope,
		&dpfaggregator.AggregatePartialReportParams{
			PartialReportURI:    *partialReportURI,
			PartialHistogramURI: *partialHistogramURI,
			DecryptedReportURI:  *decryptedReportURI,
			HelperPrivateKeys:   helperPrivKeys,
			ExpandParams:        expandParams,
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
