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
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/golang/glog"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/report/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/report/reportutils"
	"github.com/google/privacy-sandbox-aggregation-service/tools/dpfconvert"
	"github.com/google/privacy-sandbox-aggregation-service/utils/utils"
)

// TODO: Store some of the flag values in manifest files.
var (
	address              = flag.String("address", "", "Address of the server.")
	helperPublicKeysURI1 = flag.String("helper_public_keys_uri1", "", "A file that contains the public encryption key from helper1.")
	helperPublicKeysURI2 = flag.String("helper_public_keys_uri2", "", "A file that contains the public encryption key from helper2.")
	keyBitSize           = flag.Int("key_bit_size", 32, "Bit size of the conversion keys.")
	conversionURI        = flag.String("conversion_uri", "", "Input raw conversion data.")
	conversionRaw        = flag.String("conversion_raw", "2684354560,20", "Raw conversion.")
	sendCount            = flag.Int("send_count", 1, "How many times to send each conversion.")
	helperOrigin1        = flag.String("helper_origin1", "", "Origin of helper1.")
	helperOrigin2        = flag.String("helper_origin2", "", "Origin of helper2.")
	concurrency          = flag.Int("concurrency", 10, "Concurrent requests.")

	encryptOutput = flag.Bool("encrypt_output", true, "Generate reports with encryption. This should only be false for integration test before HPKE is ready in Go Tink.")

	impersonatedSvcAccount = flag.String("impersonated_svc_account", "", "Service account to impersonate, skipped if empty")

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
	log.Infof("Helper public key file locations. 1: %v, 2: %v", *helperPublicKeysURI1, *helperPublicKeysURI2)
	log.Infof("Key Bit size %v", *keyBitSize)
	log.Infof("Conversions file uri: %v", *conversionURI)
	log.Infof("Helper origins. 1: %v, 2: %v", *helperOrigin1, *helperOrigin2)

	client := retryablehttp.NewClient().StandardClient()

	ctx := context.Background()
	var token string
	var err error
	if token, err = utils.GetAuthorizationToken(ctx, *address, *impersonatedSvcAccount); err != nil {
		log.Errorf("Couldn't get Auth Bearer IdToken: %s", err)
	}

	var conversionsSent uint64
	requestCh := make(chan *bytes.Buffer)
	done := setupRequestWorkers(client, token, *concurrency, &conversionsSent, requestCh)

	helperPubKeys1, err := cryptoio.ReadPublicKeyVersions(ctx, *helperPublicKeysURI1)
	if err != nil {
		log.Exit(err)
	}
	helperPubKeys2, err := cryptoio.ReadPublicKeyVersions(ctx, *helperPublicKeysURI2)
	if err != nil {
		log.Exit(err)
	}

	// Use any version of the public keys until the version control is designed.
	var publicKeyInfo1, publicKeyInfo2 []cryptoio.PublicKeyInfo
	for _, v := range helperPubKeys1 {
		publicKeyInfo1 = v
	}
	for _, v := range helperPubKeys2 {
		publicKeyInfo2 = v
	}
	// Empty context information for demo.
	contextInfo, err := utils.MarshalCBOR(&reporttypes.SharedInfo{})
	if err != nil {
		log.Exit(err)
	}

	var conversions []reporttypes.RawReport
	if *conversionURI != "" {
		var err error
		conversions, err = reportutils.ReadRawReports(ctx, *conversionURI, *keyBitSize)
		if err != nil {
			log.Exit(err)
		}
	} else {
		conversion, err := reportutils.ParseRawReport(*conversionRaw, *keyBitSize)
		if err != nil {
			log.Exit(err)
		}
		conversions = append(conversions, conversion)
	}

	if *sendCount <= 0 {
		*sendCount = 1
	}

	for i := 0; i < *sendCount; i++ {
		for _, c := range conversions {
			report1, report2, err := dpfconvert.GenerateEncryptedReports(c, *keyBitSize, publicKeyInfo1, publicKeyInfo2, contextInfo, *encryptOutput)
			if err != nil {
				log.Exit(err)
			}
			report, err := utils.MarshalCBOR(&reporttypes.AggregationReport{
				SharedInfo: contextInfo,
				AggregationServicePayloads: []*reporttypes.AggregationServicePayload{
					{Origin: *helperOrigin1, Payload: report1.EncryptedReport.Data, KeyID: report1.KeyId},
					{Origin: *helperOrigin2, Payload: report2.EncryptedReport.Data, KeyID: report2.KeyId},
				},
			})
			if err != nil {
				log.Exit(err)
			}
			requestCh <- bytes.NewBuffer(report)
		}
	}
	close(requestCh)
	<-done
	log.Infof("All %v conversions sent!", conversionsSent)
}

func setupRequestWorkers(client *http.Client, token string, concurrency int, sent *uint64, in <-chan *bytes.Buffer) <-chan bool {
	var wg sync.WaitGroup
	done := make(chan bool)

	worker := func(in <-chan *bytes.Buffer) {
		for data := range in {
			// send request
			req, err := http.NewRequest("POST", *address, data)
			if err != nil {
				log.Error(err)
				continue
			}

			if token != "" {
				req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
			}

			req.Header.Set("Content-Type", "encrypted-report")

			resp, err := client.Do(req)
			if err != nil {
				log.Error(err)
				continue
			}
			switch resp.Status {

			case "200 OK":
				atomic.AddUint64(sent, 1)
				log.Infof("%v Conversions sent.", *sent)
			default:
				body, _ := ioutil.ReadAll(resp.Body)
				log.Infof("%v: %s", resp.Status, string(body))
			}
			resp.Body.Close()
		}
		wg.Done()
	}

	for i := 0; i < concurrency; i++ {
		go worker(in)
	}
	wg.Add(concurrency)

	go func() {
		wg.Wait()
		close(done)
	}()
	return done
}
