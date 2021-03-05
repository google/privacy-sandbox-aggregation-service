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

package conversionaggregator

import (
	"encoding/base64"
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/browsersimulator"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/conversion"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypttesting"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

type idExponentiatedKey struct {
	ReportID         string
	ExponentiatedKey *pb.ElGamalCiphertext
}

type idPartialReport struct {
	ReportID      string
	PartialReport *pb.PartialReport
}

type idAggregation struct {
	AggID       string
	Aggregation *pb.PartialAggregation
}

func calculateAggID(key, secret1, secret2 string) (string, error) {
	hased, err := elgamalencrypttesting.GetHashedECPointStrForTesting(key)
	if err != nil {
		return "", err
	}
	exp1, err := elgamalencrypt.ExponentiateOnECPointStr(hased, secret1)
	if err != nil {
		return "", err
	}
	return elgamalencrypt.ExponentiateOnECPointStr(exp1, secret2)
}

func TestAggregatePartialReport(t *testing.T) {
	priv, pub, err := elgamalencrypt.GenerateElGamalKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	secret1, err := elgamalencrypt.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	secret2, err := elgamalencrypt.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}

	conversionKey := "conversion key"
	wantAggID, err := calculateAggID(conversionKey, secret1, secret2)
	if err != nil {
		t.Fatal(err)
	}

	var exponentiatedKeyInput []idExponentiatedKey
	for _, reportID := range []string{
		"id1",
		"id2",
	} {
		encryptedKey, err := elgamalencrypt.Encrypt(conversionKey, pub)
		if err != nil {
			t.Fatal(err)
		}
		exponentiatedKey, err := elgamalencrypt.ExponentiateOnCiphertext(encryptedKey, pub, secret1)
		if err != nil {
			t.Fatal(err)
		}
		exponentiatedKeyInput = append(exponentiatedKeyInput, idExponentiatedKey{
			ReportID:         reportID,
			ExponentiatedKey: exponentiatedKey,
		})
	}

	var partialReportInput []idPartialReport
	for reportID, report := range map[string]*pb.PartialReport{
		"id1": {KeyShare: []byte("to_drop"), ValueShare: 1},
		"id2": {KeyShare: []byte("to_keep"), ValueShare: 2},
	} {
		partialReportInput = append(partialReportInput, idPartialReport{
			ReportID:      reportID,
			PartialReport: report,
		})
	}

	pipeline, scope := beam.NewPipelineWithRoot()

	rInput := beam.CreateList(scope, exponentiatedKeyInput)
	pInput := beam.CreateList(scope, partialReportInput)

	exponentiatedKey := beam.ParDo(scope, func(r idExponentiatedKey) (string, *pb.ElGamalCiphertext) {
		return r.ReportID, r.ExponentiatedKey
	}, rInput)
	partialReport := beam.ParDo(scope, func(p idPartialReport) (string, *pb.PartialReport) {
		return p.ReportID, p.PartialReport
	}, pInput)

	aggIDKeyShare, aggData := conversion.RekeyByAggregationID(scope, exponentiatedKey, partialReport, priv, secret2)
	partialAggregation := AggregateDataShare(scope, aggIDKeyShare, aggData, true /*ignorePrivacy*/, PrivacyParams{})

	got := beam.ParDo(scope, func(aggID string, result *pb.PartialAggregation) idAggregation {
		return idAggregation{AggID: base64.StdEncoding.EncodeToString([]byte(aggID)), Aggregation: result}
	}, partialAggregation)

	passert.Equals(scope, got, idAggregation{AggID: base64.StdEncoding.EncodeToString([]byte(wantAggID)), Aggregation: &pb.PartialAggregation{
		KeyShare:     []byte("to_keep"),
		PartialCount: 2,
		PartialSum:   3,
	}})

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

func toTableFn(idAgg idAggregation) (string, *pb.PartialAggregation) {
	return idAgg.AggID, idAgg.Aggregation
}

func TestMergeAggregation(t *testing.T) {
	wantKey := "want key"
	keyShare1, keyShare2, err := browsersimulator.SplitIntoByteShares([]byte(wantKey))
	if err != nil {
		t.Fatal(err)
	}

	pipeline, scope := beam.NewPipelineWithRoot()
	partialAgg1 := beam.ParDo(scope, toTableFn, beam.CreateList(scope, []idAggregation{
		{AggID: "keep", Aggregation: &pb.PartialAggregation{KeyShare: keyShare1, PartialCount: 1, PartialSum: 1}},
		{AggID: "drop", Aggregation: &pb.PartialAggregation{KeyShare: keyShare1, PartialCount: 1, PartialSum: 1}},
	}))
	partialAgg2 := beam.ParDo(scope, toTableFn, beam.CreateList(scope, []idAggregation{
		{AggID: "keep", Aggregation: &pb.PartialAggregation{KeyShare: keyShare2, PartialCount: 2, PartialSum: 2}},
	}))
	gotResult := MergeAggregation(scope, partialAgg1, partialAgg2)

	passert.Equals(scope, gotResult, CompleteResult{ConversionKey: wantKey, Sum: 3, Count: 3})
	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}
