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

package onepartyaggregator

import (
	"context"
	"encoding/base64"
	"math/rand"
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"lukechampine.com/uint128"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/standardencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/pipelinetypes"
	"github.com/google/privacy-sandbox-aggregation-service/shared/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"

	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"
)

func getRandomPublicKey(keys []cryptoio.PublicKeyInfo) (string, *pb.StandardPublicKey, error) {
	keyInfo := keys[rand.Intn(len(keys))]
	bKey, err := base64.StdEncoding.DecodeString(keyInfo.Key)
	if err != nil {
		return "", nil, err
	}
	return keyInfo.ID, &pb.StandardPublicKey{Key: bKey}, nil
}

type encryptReportFn struct {
	PublicKeys []cryptoio.PublicKeyInfo
}

func (fn *encryptReportFn) ProcessElement(ctx context.Context, report *pipelinetypes.RawReport, emit func(*pb.AggregatablePayload)) error {
	payload := reporttypes.Payload{
		Operation: "one-party",
		Data: []reporttypes.Contribution{
			{Bucket: utils.Uint128ToBigEndianBytes(report.Bucket), Value: utils.Uint32ToBigEndianBytes(uint32(report.Value))},
		},
	}
	bPayload, err := utils.MarshalCBOR(payload)
	if err != nil {
		return err
	}

	keyID, key, err := getRandomPublicKey(fn.PublicKeys)
	if err != nil {
		return err
	}
	encrypted, err := standardencrypt.Encrypt(bPayload, nil, key)
	if err != nil {
		return err
	}
	emit(&pb.AggregatablePayload{Payload: encrypted, SharedInfo: "", KeyId: keyID})

	return nil
}

type standardEncryptFn struct {
	PublicKeys []cryptoio.PublicKeyInfo
}

func (fn *standardEncryptFn) ProcessElement(report *pipelinetypes.RawReport, emit func(*pb.AggregatablePayload)) error {
	payload := &reporttypes.Payload{
		Data: []reporttypes.Contribution{
			{Bucket: utils.Uint128ToBigEndianBytes(report.Bucket), Value: utils.Uint32ToBigEndianBytes(uint32(report.Value))},
		},
	}

	bPayload, err := utils.MarshalCBOR(payload)
	if err != nil {
		return err
	}

	sharedInfo := "context"
	keyID, publicKey, err := getRandomPublicKey(fn.PublicKeys)
	if err != nil {
		return err
	}
	result, err := standardencrypt.Encrypt(bPayload, []byte(sharedInfo), publicKey)
	if err != nil {
		return err
	}
	emit(&pb.AggregatablePayload{Payload: result, SharedInfo: sharedInfo, KeyId: keyID})
	return nil
}

func TestDecryptPartialReport(t *testing.T) {
	ctx := context.Background()
	privKeys, pubKeysInfo, err := cryptoio.GenerateHybridKeyPairs(ctx, 1, "", "")
	if err != nil {
		t.Fatal(err)
	}

	reports := []*pipelinetypes.RawReport{
		{Bucket: uint128.From64(1), Value: 1},
		{Bucket: uint128.From64(2), Value: 2},
	}

	pipeline, scope := beam.NewPipelineWithRoot()

	wantReports := beam.CreateList(scope, reports)
	encryptedReports := beam.ParDo(scope, &standardEncryptFn{PublicKeys: pubKeysInfo}, wantReports)
	got := DecryptReport(scope, encryptedReports, privKeys)
	gotReports := beam.ParDo(scope, func(key uint128.Uint128, value uint64) *pipelinetypes.RawReport {
		return &pipelinetypes.RawReport{Bucket: key, Value: value}
	}, got)

	passert.Equals(scope, gotReports, wantReports)
	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

type keyValue struct {
	Key   uint128.Uint128
	Value uint64
}

func TestAggregationPipelineOneParty(t *testing.T) {
	ctx := context.Background()
	privKeys, pubKeysInfo, err := cryptoio.GenerateHybridKeyPairs(ctx, 10, "", "")
	if err != nil {
		t.Fatal(err)
	}

	var rawReports []*pipelinetypes.RawReport
	wantSum := make(map[uint128.Uint128]uint64)
	for i := 5; i <= 20; i++ {
		for j := 0; j < i; j++ {
			index := uint128.From64(uint64(i) << 27)
			value := uint64(i)
			wantSum[index] += value
			rawReports = append(rawReports, &pipelinetypes.RawReport{Bucket: index, Value: value})
		}
	}
	var wantResult []*keyValue
	for key := range wantSum {
		wantResult = append(wantResult, &keyValue{Key: key, Value: wantSum[key]})
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	report := beam.CreateList(scope, rawReports)
	encrypted := beam.ParDo(scope, &encryptReportFn{PublicKeys: pubKeysInfo}, report)

	decrypted := DecryptReport(scope, encrypted, privKeys)
	result := SumRawReport(scope, decrypted)
	got := beam.ParDo(scope, func(index uint128.Uint128, value uint64) *keyValue {
		return &keyValue{Key: index, Value: value}
	}, result)

	want := beam.CreateList(scope, wantResult)
	passert.Equals(scope, got, want)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func TestFilterResults(t *testing.T) {
	targetBuckets := map[int]bool{
		5: true,
		6: true,
	}

	var rawReports []*pipelinetypes.RawReport
	wantSum := make(map[uint128.Uint128]uint64)
	for i := 5; i <= 20; i++ {
		for j := 0; j < i; j++ {
			index := uint128.From64(uint64(i) << 27)
			value := uint64(i)
			rawReports = append(rawReports, &pipelinetypes.RawReport{Bucket: index, Value: value})
			if _, ok := targetBuckets[i]; ok {
				wantSum[index] += value
			}
		}
	}
	var wantResult []*keyValue
	for key := range wantSum {
		wantResult = append(wantResult, &keyValue{Key: key, Value: wantSum[key]})
	}

	type keyBool struct {
		Key   uint128.Uint128
		Value bool
	}
	var bucketList []*keyBool
	for i, v := range targetBuckets {
		index := uint128.From64(uint64(i) << 27)
		bucketList = append(bucketList, &keyBool{Key: index, Value: v})
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	bucket := beam.CreateList(scope, bucketList)
	filterBucket := beam.ParDo(scope, func(v *keyBool) (uint128.Uint128, bool) {
		return v.Key, v.Value
	}, bucket)
	report := beam.CreateList(scope, rawReports)
	reportTable := beam.ParDo(scope, func(r *pipelinetypes.RawReport) (uint128.Uint128, uint64) {
		return r.Bucket, r.Value
	}, report)

	rawResult := SumRawReport(scope, reportTable)
	joined := beam.CoGroupByKey(scope, filterBucket, rawResult)
	result := beam.ParDo(scope, &filterBucketFn{}, joined)

	got := beam.ParDo(scope, func(index uint128.Uint128, value uint64) *keyValue {
		return &keyValue{Key: index, Value: value}
	}, result)

	want := beam.CreateList(scope, wantResult)
	passert.Equals(scope, got, want)

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}
