package main

import (
	"context"
	"flag"
	"fmt"

	log "github.com/golang/glog"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfdataconverter"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/service/query"
)

var (
	rawReportURI       = flag.String("raw_report_uri", "", "Input raw report data.")
	expansionConfigURI = flag.String("expansion_config_uri", "", "URI for the expansion configurations that defines the query hierarchy.")
	keyBitSize         = flag.Int("key_bit_size", 20, "Bit size of the data bucket keys. Support up to 128 bit.")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	config, err := query.ReadExpansionConfigFile(ctx, *expansionConfigURI)
	if err != nil {
		log.Fatal(err)
	}

	buckets, err := readReportBuckets(ctx, *rawReportURI, *keyBitSize)
	if err != nil {
		log.Fatal(err)
	}

	for _, prefixLength := range config.PrefixLengths {
		prefixes := getUniquePrefixes(buckets, uint64(prefixLength), uint64(*keyBitSize))
		fmt.Printf("unique prefix count: %d, for prefix length: %d", len(prefixes), prefixLength)
	}
}

func readReportBuckets(ctx context.Context, uri string, keyBitSize int) ([]uint64, error) {
	lines, err := ioutils.ReadLines(ctx, uri)
	if err != nil {
		return nil, err
	}
	var buckets []uint64
	for _, l := range lines {
		r, err := dpfdataconverter.ParseRawConversion(l, keyBitSize)
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, r.Bucket.Lo)
	}
	return buckets, nil
}

func getPrefix(val, prefixLength, keyBitSize uint64) uint64 {
	return val >> (keyBitSize - prefixLength)
}

func getUniquePrefixes(buckets []uint64, prefixLength, keyBitSize uint64) []uint64 {
	prefixes := make(map[uint64]bool)
	for _, b := range buckets {
		prefixes[getPrefix(b, prefixLength, keyBitSize)] = true
	}
	var output []uint64
	for p := range prefixes {
		output = append(output, p)
	}
	return output
}
