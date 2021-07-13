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

// This binary reads raw conversions from an input file, creates and encrypts partial reports.
// Then send the reports to a HTTP endpoint run by the ad-tech.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"time"

	log "github.com/golang/glog"
	"cloud.google.com/go/compute/metadata"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfbrowsersimulator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/service/collectorservice"
)

// TODO: Store some of the flag values in manifest files.
var (
	address              = flag.String("address", "", "Address of the server.")
	helperPublicKeyFile1 = flag.String("helper_public_key_file1", "", "A file that contains the public encryption key from helper1.")
	helperPublicKeyFile2 = flag.String("helper_public_key_file2", "", "A file that contains the public encryption key from helper2.")
	keyBitSize           = flag.Int("key_bit_size", 32, "Bit size of the conversion keys.")
	conversionFile       = flag.String("conversion_file", "", "Input raw conversion data.")
	helperOrigin1        = flag.String("helper_origin1", "", "Origin of helper1.")
	helperOrigin2        = flag.String("helper_origin2", "", "Origin of helper2.")

	version string // set by linker -X
	build   string // set by linker -X
)

func main() {
	flag.Parse()

	buildDate := time.Unix(0, 0)
	if i, err := strconv.ParseInt(build, 10, 64); err != nil {
		log.Error(err)
	} else {
		buildDate = time.Unix(i, 0)
	}

	log.Info("- Debugging enabled - \n")
	log.Infof("Running browser simulator version: %v, build: %v\n", version, buildDate)

	log.Infof("Requests sent to %v", *address)
	log.Infof("Helper public key file locations. 1: %v, 2: %v", *helperPublicKeyFile1, *helperPublicKeyFile2)
	log.Infof("Key Bit size %v", *keyBitSize)
	log.Infof("Conversions file uri: %v", *conversionFile)
	log.Infof("Helper origins. 1: %v, 2: %v", *helperOrigin1, *helperOrigin2)

	tokenURL := fmt.Sprintf("/instance/service-accounts/default/identity?audience=%s", *address)
	idToken, err := metadata.Get(tokenURL)
	if err != nil {
		log.Exit("metadata.Get: failed to query id_token: %+v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{},
	}

	publicKey1, err := cryptoio.ReadStandardPublicKey(*helperPublicKeyFile1)
	if err != nil {
		log.Exit(err)
	}
	publicKey2, err := cryptoio.ReadStandardPublicKey(*helperPublicKeyFile2)
	if err != nil {
		log.Exit(err)
	}

	// Empty context information for demo.
	contextInfo, err := ioutils.MarshalCBOR(&collectorservice.SharedInfo{})
	if err != nil {
		log.Exit(err)
	}

	ctx := context.Background()
	conversions, err := dpfbrowsersimulator.ReadRawConversions(ctx, *conversionFile, int32(*keyBitSize))
	if err != nil {
		log.Exit(err)
	}

	for _, c := range conversions {
		report1, report2, err := dpfbrowsersimulator.GenerateEncryptedReports(c, int32(*keyBitSize), publicKey1, publicKey2, contextInfo)
		if err != nil {
			log.Exit(err)
		}
		report, err := ioutils.MarshalCBOR(&collectorservice.AggregationReport{
			SharedInfo: contextInfo,
			Payloads: []*collectorservice.AggregationServicePayload{
				{Origin: *helperOrigin1, Payload: report1.EncryptedReport.Data, KeyID: "example_key_id"},
				{Origin: *helperOrigin2, Payload: report2.EncryptedReport.Data, KeyID: "example_key_id"},
			},
		})
		if err != nil {
			log.Exit(err)
		}

		req, err := http.NewRequest("POST", *address, bytes.NewBuffer(report))
		if err != nil {
			log.Exit(err)
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", idToken))
		req.Header.Set("Content-Type", "encrypted-report")

		_, err = client.Do(req)
		if err != nil {
			log.Exit(err)
		}
	}
}
