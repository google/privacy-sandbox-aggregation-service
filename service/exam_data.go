package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	log "github.com/golang/glog"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfaggregator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/dpfdataconverter"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/service/query"
)

var (
	rawReportURI       = flag.String("raw_report_uri", "", "Input raw report data.")
	expansionConfigURI = flag.String("expansion_config_uri", "", "URI for the expansion configurations that defines the query hierarchy.")
	keyBitSize         = flag.Int("key_bit_size", 20, "Bit size of the data bucket keys. Support up to 128 bit.")

	mergedResultURI = flag.String("merged_result_uri", "", "Merged result.")
	expandParamsURI = flag.String("expand_params_uri", "", "URI for the output expand parameters.")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	// analyzeInput(*expansionConfigURI, *rawReportURI, *keyBitSize)
	if err := createExpandParams(ctx, *mergedResultURI, *expandParamsURI, *keyBitSize); err != nil {
		log.Fatal(err)
	}
}

func createExpandParams(ctx context.Context, resultURI, expandParamsURI string, keyBitSize int) error {
	keys, err := getUniqeKeys(ctx, resultURI, keyBitSize)
	if err != nil {
		return err
	}

	dedupedKeys := dedupKeys(keys)

	return dpfaggregator.SaveExpandParameters(ctx, &dpfaggregator.ExpandParameters{
		Levels:        []int32{int32(keyBitSize - 1)},
		PreviousLevel: -1,
		Prefixes:      [][]uint128.Uint128{dedupedKeys},
	}, expandParamsURI)
}

func dedupKeys(keys []uint128.Uint128) []uint128.Uint128 {
	dedup := make(map[uint128.Uint128]bool)
	for _, k := range keys {
		if ok := dedup[k]; !ok {
			dedup[k] = true
		} else {
			fmt.Println("duplication found!!!!")
		}
	}

	var output []uint128.Uint128
	for k := range dedup {
		output = append(output, k)
	}
	fmt.Printf("input count %d vs output count %d \n", len(keys), len(output))
	return output
}

func getUniqeKeys(ctx context.Context, resultURI string, keyBitSize int) ([]uint128.Uint128, error) {
	lines, err := ioutils.ReadLines(ctx, resultURI)
	if err != nil {
		return nil, err
	}

	var values []uint128.Uint128
	for _, line := range lines {
		cols := strings.Split(line, ",")
		if got, want := len(cols), 2; got != want {
			return nil, fmt.Errorf("got %d columns in line %q, want %d", got, line, want)
		}

		key128, err := ioutils.StringToUint128(cols[0])
		if err != nil {
			return nil, err
		}
		if key128.Cmp(getMaxKey(keyBitSize)) == 1 {
			return nil, fmt.Errorf("key %q overflows the integer with %d bits", key128.String(), keyBitSize)
		}
		values = append(values, key128)
	}
	return values, nil
}

func analyzeInput(ctx context.Context, expansionConfigURI, rawReportURI string, keyBitSize int) {
	config, err := query.ReadExpansionConfigFile(ctx, expansionConfigURI)
	if err != nil {
		log.Fatal(err)
	}

	buckets, err := readReportBuckets(ctx, rawReportURI, keyBitSize)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("input: %q\n", rawReportURI)
	var lastNonzeroCount int
	for i, prefixLength := range config.PrefixLengths {
		nonzeros := getUniquePrefixes(buckets, uint64(prefixLength), uint64(keyBitSize))

		vecLen := 1 << prefixLength
		if i > 0 {
			vecLen = lastNonzeroCount * int(1<<(prefixLength-config.PrefixLengths[i-1]))
		}
		fmt.Printf("level %d, prefix length: %d, non-zero count: %d out of domain %d, expanded vector length: %d\n", i, prefixLength, len(nonzeros), 1<<prefixLength, vecLen)
		lastNonzeroCount = len(nonzeros)
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

func getMaxKey(s int) uint128.Uint128 {
	maxKey := uint128.Max
	if s < 128 {
		maxKey = uint128.Uint128{1, 0}.Lsh(uint(s)).Sub64(1)
	}
	return maxKey
}
