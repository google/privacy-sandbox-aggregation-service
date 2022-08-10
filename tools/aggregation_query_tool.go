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

// This binary queries the aggregation results by publishing the requests on certain PubSub topics.
package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"time"

	log "github.com/golang/glog"
	"cloud.google.com/go/pubsub"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/pborman/uuid"
	"github.com/google/privacy-sandbox-aggregation-service/service/aggregatorservice"
	"github.com/google/privacy-sandbox-aggregation-service/service/query"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"
)

var (
	helperAddress1     = flag.String("helper_address1", "", "Address of helper 1.")
	helperAddress2     = flag.String("helper_address2", "", "Address of helper 2, required for MPC protocal.")
	partialReportURI1  = flag.String("partial_report_uri1", "", "Input partial report for helper 1.")
	partialReportURI2  = flag.String("partial_report_uri2", "", "Input partial report for helper 2, required for MPC protocal.")
	expansionConfigURI = flag.String("expansion_config_uri", "", "URI for the expansion configurations with type query.HierarchicalConfig, query.DirectConfig or a single column of bucket IDs for the one-party design.")
	epsilon            = flag.Float64("epsilon", 0.0, "Total privacy budget for the hierarchical query. For experiments, no noise will be added when epsilon is zero.")
	keyBitSize         = flag.Int("key_bit_size", 32, "Bit size of the data bucket keys. Support up to 128 bit.")
	resultDir          = flag.String("result_dir", "", "The directory where the final results will be saved. Helpers should only have writing permissions to this directory.")
	aggType            = flag.String("agg_type", "conversion", "Aggregation type, should be 'conversion' or 'reach'.")

	impersonatedSvcAccount = flag.String("impersonated_svc_account", "", "Service account to impersonate, skipped if empty")

	numWorkers = flag.Int("num_workers", 1, "Initial number of workers for Dataflow job")

	version string // set by linker -X
	build   string // set by linker -X
)

func main() {
	flag.Parse()

	buildDate := time.Unix(0, 0)
	if i, err := strconv.ParseInt(build, 10, 64); err != nil {
		log.Info(err)
	} else {
		buildDate = time.Unix(i, 0)
	}

	log.Info("- Debugging enabled - \n")
	log.Infof("Running querier simulator version: %v, build: %v\n", version, buildDate)

	if *aggType == "" {
		log.Exit("aggregation type empty")
	}

	ctx := context.Background()
	client := retryablehttp.NewClient().StandardClient()
	queryID := uuid.New()

	var (
		token1, token2               string
		sharedInfo1, sharedInfo2     *query.HelperSharedInfo
		pubsubClient1, pubsubClient2 *pubsub.Client
		project1, project2           string
		topic1, topic2               string
		err                          error
		inputExist                   bool
	)

	inputExist, err = utils.IsFileGlobExist(ctx, *partialReportURI1)
	if err != nil {
		log.Exit(err)
	} else if !inputExist {
		log.Exitf("input not found: %q", *partialReportURI1)
	}

	if token1, err = utils.GetAuthorizationToken(ctx, *helperAddress1, *impersonatedSvcAccount); err != nil {
		log.Infof("Couldn't get Auth Bearer IdToken: %s", err)
	}

	sharedInfo1, err = aggregatorservice.ReadHelperSharedInfo(client, *helperAddress1, token1)
	if err != nil {
		log.Exit(err)
	}

	project1, topic1, err = utils.ParsePubSubResourceName(sharedInfo1.PubSubTopic)
	if err != nil {
		log.Exit(err)
	}
	pubsubClient1, err = pubsub.NewClient(ctx, project1)
	if err != nil {
		log.Exit(err)
	}
	defer pubsubClient1.Close()

	if *helperAddress2 != "" {
		inputExist, err = utils.IsFileGlobExist(ctx, *partialReportURI2)
		if err != nil {
			log.Exit(err)
		} else if !inputExist {
			log.Exitf("input not found: %q", *partialReportURI2)
		}

		if token2, err = utils.GetAuthorizationToken(ctx, *helperAddress2, *impersonatedSvcAccount); err != nil {
			log.Infof("Couldn't get Auth Bearer IdToken: %s", err)
		}

		sharedInfo2, err = aggregatorservice.ReadHelperSharedInfo(client, *helperAddress2, token2)
		if err != nil {
			log.Exit(err)
		}
		project2, topic2, err = utils.ParsePubSubResourceName(sharedInfo2.PubSubTopic)
		if err != nil {
			log.Exit(err)
		}
		pubsubClient2, err = pubsub.NewClient(ctx, project2)
		if err != nil {
			log.Exit(err)
		}
		defer pubsubClient2.Close()
	}

	// Request aggregation on helper1.
	if err := utils.PublishRequest(ctx, pubsubClient1, topic1, &query.AggregateRequest{
		AggregationType:   *aggType,
		PartialReportURI:  *partialReportURI1,
		ExpandConfigURI:   *expansionConfigURI,
		TotalEpsilon:      *epsilon,
		QueryID:           queryID,
		PartnerSharedInfo: sharedInfo2,
		ResultDir:         *resultDir,
		KeyBitSize:        int32(*keyBitSize),
		NumWorkers:        int32(*numWorkers),
	}); err != nil {
		log.Exit(err)
	}

	if *helperAddress2 != "" {
		// Request aggregation on helper2.
		if err := utils.PublishRequest(ctx, pubsubClient2, topic2, &query.AggregateRequest{
			AggregationType:   *aggType,
			PartialReportURI:  *partialReportURI2,
			ExpandConfigURI:   *expansionConfigURI,
			TotalEpsilon:      *epsilon,
			QueryID:           queryID,
			PartnerSharedInfo: sharedInfo1,
			ResultDir:         *resultDir,
			KeyBitSize:        int32(*keyBitSize),
			NumWorkers:        int32(*numWorkers),
		}); err != nil {
			log.Exit(err)
		}
	}

	fmt.Printf("query request sent with ID %q", queryID)
}
