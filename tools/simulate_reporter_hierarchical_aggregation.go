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

// This binary simulates the report origins sending requests to helpers for aggregating partial reports.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"

	log "github.com/golang/glog"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc"
	"github.com/google/privacy-sandbox-aggregation-service/service/query"
)

var (
	helperAddr1            = flag.String("helper_addr1", "", "Address of helper 1.")
	helperAddr2            = flag.String("helper_addr2", "", "Address of helper 2.")
	insecure               = flag.Bool("insecure", false, "Insecure helper connections - default false")
	impersonatedSvcAccount = flag.String("impersonated_svc_account", "", "Service account to impersonate, skipped if empty")

	partialReportURI1        = flag.String("partial_report_uri1", "", "Input partial report for helper 1.")
	partialReportURI2        = flag.String("partial_report_uri2", "", "Input partial report for helper 2.")
	hierarchicalHistogramURI = flag.String("hierarchical_histogram_uri", "", "Output file for the hierarchical aggregation results.")
	expansionConfigURI       = flag.String("expansion_config_uri", "", "Input file for the expansion configurations that defines the query hierarchy.")
	paramsDir                = flag.String("params_dir", "", "Input directory that stores the parameter files.")
	partialAggregationDir    = flag.String("partial_aggregation_dir", "", "Output directory for the partial aggregation files.")

	epsilon    = flag.Float64("epsilon", 0.0, "Total privacy budget for the hierarchical query. For experiments, no noise will be added when epsilon is zero.")
	keyBitSize = flag.Int("key_bit_size", 32, "Bit size of the conversion keys.")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	expansionConfig, err := query.ReadExpansionConfigFile(ctx, *expansionConfigURI)
	if err != nil {
		log.Exit(err)
	}
	systemRoots, err := x509.SystemCertPool()
	if err != nil {
		log.Exit(err)
	}
	cred := credentials.NewTLS(&tls.Config{
		RootCAs: systemRoots,
	})
	// grpc.WithInsecure() is used for demonstration, and for real instances we should use more secure options.
	var option grpc.DialOption
	if *insecure {
		option = grpc.WithInsecure()
	} else {
		option = grpc.WithTransportCredentials(cred)
	}

	conn1, err := grpc.Dial(*helperAddr1, option)
	if err != nil {
		log.Exit(err)
	}
	defer conn1.Close()

	conn2, err := grpc.Dial(*helperAddr2, option)
	if err != nil {
		log.Exit(err)
	}
	defer conn2.Close()

	prefixQuery := &query.PrefixHistogramQuery{
		PartialReportURI1:      *partialReportURI1,
		PartialReportURI2:      *partialReportURI2,
		PartialAggregationDir:  *partialAggregationDir,
		ParamsDir:              *paramsDir,
		Helper1:                conn1,
		Helper2:                conn2,
		ImpersonatedSvcAccount: *impersonatedSvcAccount,
		KeyBitSize:             int32(*keyBitSize),
	}

	results, err := prefixQuery.HierarchicalAggregation(ctx, *epsilon, expansionConfig)
	if err != nil {
		log.Exit(err)
	}
	if err := query.WriteHierarchicalResultsFile(ctx, results, *hierarchicalHistogramURI); err != nil {
		log.Exit(err)
	}
}
