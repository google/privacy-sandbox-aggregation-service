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

// This binary reads the partial report and reencrypts the conversion keys for the other helper.
// The pipeline can be executed in two ways:
//
// 1. Directly on local
// /path/to/exponentiate_conversion_key \
// --partial_report_file=/path/to/partial_report_file.txt \
// --reencrypted_key_file=/path/to/reencrypted_key_file.txt \
// --private_key_dir=/path/to/private_key_dir \
// --other_public_key_dir=/path/to/other_public_key_dir \
// --runner=direct
//
// 2. Dataflow on cloud
// /path/to/exponentiate_conversion_key \
// --partial_report_file=gs://<helper bucket>/partial_report_file.txt \
// --reencrypted_key_file=gs://<helper bucket>/reencrypted_key_file.txt \
// --private_key_dir=/path/to/private_key_dir \
// --other_public_key_dir=/path/to/other_public_key_dir \
// --runner=dataflow \
// --project=<GCP project> \
// --temp_location=gs://<dataflow temp dir> \
// --staging_location=gs://<dataflow temp dir> \
// --worker_binary=/path/to/reencrypt_conversion_key

package main

import (
	"context"
	"flag"
	"path"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/log"
	"github.com/apache/beam/sdks/go/pkg/beam/x/beamx"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/conversion"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
)

var (
	partialReportFile  = flag.String("partial_report_file", "", "Input partial reports.")
	reencryptedKeyFile = flag.String("reencrypted_key_file", "", "Output reencrypted conversion keys.")
	privateKeyDir      = flag.String("private_key_dir", "", "Directory for private keys and exponential secret.")
	otherPublicKeyDir  = flag.String("other_public_key_dir", "", "Directory for the ElGamal public key from the other helper.")
	fileShards         = flag.Int64("file_shards", 1, "The number of shards for the output file.")
)

func main() {
	flag.Parse()

	beam.Init()

	ctx := context.Background()
	helperInfo, err := conversion.GetPrivateInfo(*privateKeyDir)
	if err != nil {
		log.Exit(ctx, err)
	}
	otherPublicKey, err := cryptoio.ReadElGamalPublicKey(path.Join(*otherPublicKeyDir, cryptoio.DefaultElgamalPublicKey))
	if err != nil {
		log.Exit(ctx, err)
	}

	pipeline := beam.NewPipeline()
	scope := pipeline.Root()

	conversion.ExponentiateConversionKey(scope, *partialReportFile, *reencryptedKeyFile, helperInfo, otherPublicKey, *fileShards)
	if err := beamx.Run(ctx, pipeline); err != nil {
		log.Exitf(ctx, "Failed to execute job: %s", err)
	}
}
