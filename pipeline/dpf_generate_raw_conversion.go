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

// This binary generates fake raw conversions for aggregation experiments on the hierarchical DPF expansion.
// The distribution of the conversion IDs are controlled by a prefix tree structure,
// which also creates DPF parameters that determine the hierarchy how the DPF keys are generated and expanded.
package main

import (
	"context"
	"flag"
	"fmt"

	log "github.com/golang/glog"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfbrowsersimulator"
)

var (
	sumParamsOutputFile     = flag.String("sum_params_output_file", "", "Output file for the DPF parameters for SUM.")
	prefixesOutPutFile      = flag.String("prefixes_output_file", "", "Output file for the prefixes.")
	rawConversionOutputFile = flag.String("raw_conversion_output_file", "", "Output file for the fake raw conversions.")
	totalCount              = flag.Uint64("total_count", 1000000, "Total count of raw conversions.")

	logN                = flag.Uint64("log_n", 20, "Bits of the aggregation domain size.")
	logElementSizeSum   = flag.Uint64("log_element_size_sum", 6, "Bits of element size for SUM aggregation.")
	logElementSizeCount = flag.Uint64("log_element_size_count", 6, "Bits of element size for COUNT aggregation.")
)

func writeConversions(filename string, conversions []dpfbrowsersimulator.RawConversion) error {
	lines := make([]string, len(conversions))
	for i, conversion := range conversions {
		lines[i] = fmt.Sprintf("%d,%d", conversion.Index, conversion.Value)
	}
	return cryptoio.SaveLines(filename, lines)
}

func main() {
	flag.Parse()

	// Create the prefix tree.
	root := &dpfbrowsersimulator.PrefixNode{Class: "root"}
	// Suppose the first 12 bits represent the campaign ID, and only 2^5 IDs have data.
	for i := 0; i < 1<<5; i++ {
		child := root.AddChildNode("campaignid", 12 /*bitSize*/, uint64(i) /*value*/)
		// Following 5 bits representing geo, and only 2^3 locations have data.
		for j := 0; j < 1<<3; j++ {
			child.AddChildNode("geo", 5 /*bitSize*/, uint64(j) /*value*/)
		}
	}

	prefixes, prefixDomainBits := dpfbrowsersimulator.CalculatePrefixes(root)
	sumParams := dpfbrowsersimulator.CalculateParameters(prefixDomainBits, int32(*logN), 1<<*logElementSizeSum)
	ctx := context.Background()
	if err := cryptoio.SavePrefixes(ctx, *prefixesOutPutFile, prefixes); err != nil {
		log.Exit(err)
	}
	if err := cryptoio.SaveDPFParameters(ctx, *sumParamsOutputFile, sumParams); err != nil {
		log.Exit(err)
	}

	var conversions []dpfbrowsersimulator.RawConversion
	for i := uint64(0); i < *totalCount; i++ {
		index, err := dpfbrowsersimulator.CreateConversionIndex(prefixes.Prefixes[len(prefixes.Prefixes)-1].Prefix, prefixDomainBits[len(prefixDomainBits)-1], *logN, true /*hasPrefix*/)
		if err != nil {
			log.Exit(err)
		}
		conversions = append(conversions, dpfbrowsersimulator.RawConversion{Index: index, Value: 1})
	}
	if err := writeConversions(*rawConversionOutputFile, conversions); err != nil {
		log.Exit(err)
	}
}
