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

package conversion

import (
	"testing"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/passert"
	"github.com/apache/beam/sdks/go/pkg/beam/testing/ptest"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypttesting"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"

	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"
)

type idReport struct {
	ID     string
	Report *pb.PartialReport
}

type idDecrypted struct {
	ID string
	// It seems binary data should be compared based on []byte type. Comparing them based on strings fails sometimes.
	Decrypted []byte
}

type deencryptKeyFn struct {
	PrivateKey *pb.ElGamalPrivateKey
}

func (fn *deencryptKeyFn) ProcessElement(reportID string, encrypted *pb.ElGamalCiphertext, emit func(idDecrypted)) error {
	decrypted, err := elgamalencrypt.Decrypt(encrypted, fn.PrivateKey)
	if err != nil {
		return err
	}
	emit(idDecrypted{ID: reportID, Decrypted: []byte(decrypted)})
	return nil
}

func TestExponentiateKey(t *testing.T) {
	priv, pub, err := elgamalencrypt.GenerateElGamalKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	secret, err := elgamalencrypt.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}

	const key = "key"
	encryptedKey, err := elgamalencrypt.Encrypt(key, pub)
	if err != nil {
		t.Fatal(err)
	}

	hashed, err := elgamalencrypttesting.GetHashedECPointStrForTesting(key)
	if err != nil {
		t.Fatal(err)
	}
	directExp, err := elgamalencrypt.ExponentiateOnECPointStr(hashed, secret)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, scope := beam.NewPipelineWithRoot()

	input := beam.CreateList(scope, []idReport{
		{ID: "test_id", Report: &pb.PartialReport{EncryptedConversionKey: encryptedKey}},
	})
	partialReport := beam.ParDo(scope, func(ir idReport) (string, *pb.PartialReport) {
		return ir.ID, ir.Report
	}, input)

	reencrypted := beam.ParDo(scope, &exponentiateKeyFn{Secret: secret, ElGamalPublicKey: pub}, partialReport)
	decrypted := beam.ParDo(scope, &deencryptKeyFn{
		PrivateKey: priv,
	}, reencrypted)

	passert.Equals(scope, decrypted, idDecrypted{ID: "test_id", Decrypted: []byte(directExp)})

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}

type idKey struct {
	ID           string
	EncryptedKey *pb.ElGamalCiphertext
}

type compareIDKeyShare struct {
	AggID    []byte
	ReportID string
	KeyShare string
}

type compareAggData struct {
	ReportID   string
	AggID      []byte
	ValueShare int64
}

func TestRekeyByAggregationID(t *testing.T) {
	priv, pub, err := elgamalencrypt.GenerateElGamalKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	s1, err := elgamalencrypt.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	s2, err := elgamalencrypt.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}

	conversionKey := "conversion key"
	encrypted, err := elgamalencrypt.Encrypt(conversionKey, pub)
	if err != nil {
		t.Fatal(err)
	}
	hashed, err := elgamalencrypttesting.GetHashedECPointStrForTesting(conversionKey)
	if err != nil {
		t.Fatal(err)
	}
	exp1, err := elgamalencrypt.ExponentiateOnECPointStr(hashed, s1)
	if err != nil {
		t.Fatal(err)
	}
	wantAggID, err := elgamalencrypt.ExponentiateOnECPointStr(exp1, s2)
	if err != nil {
		t.Fatal(err)
	}

	exponentiated, err := elgamalencrypt.ExponentiateOnCiphertext(encrypted, pub, s1)
	if err != nil {
		t.Fatal(err)
	}

	pipeline, scope := beam.NewPipelineWithRoot()

	externalKeyInput := beam.CreateList(scope, []idKey{
		{ID: "mutual_id", EncryptedKey: exponentiated},
	})
	externalKey := beam.ParDo(scope, func(k idKey) (string, *pb.ElGamalCiphertext) {
		return k.ID, k.EncryptedKey
	}, externalKeyInput)

	reportInput := beam.CreateList(scope, []idReport{
		{ID: "mutual_id", Report: &pb.PartialReport{ValueShare: uint32(123), KeyShare: []byte("share")}},
	})
	report := beam.ParDo(scope, func(k idReport) (string, *pb.PartialReport) {
		return k.ID, k.Report
	}, reportInput)
	gotIDKeyOutput, gotAggDataOutput := RekeyByAggregationID(scope, externalKey, report, priv, s2)
	gotIDKey := beam.ParDo(scope, func(aggID string, idKeyShare IDKeyShare) compareIDKeyShare {
		return compareIDKeyShare{AggID: []byte(aggID), ReportID: idKeyShare.ReportID, KeyShare: string(idKeyShare.KeyShare)}
	}, gotIDKeyOutput)
	gotAggData := beam.ParDo(scope, func(aggData AggData) compareAggData {
		return compareAggData{ReportID: aggData.ReportID, AggID: []byte(aggData.AggID), ValueShare: aggData.ValueShare}
	}, gotAggDataOutput)

	passert.Equals(scope, gotIDKey, compareIDKeyShare{AggID: []byte(wantAggID), ReportID: "mutual_id", KeyShare: "share"})
	passert.Equals(scope, gotAggData, compareAggData{ReportID: "mutual_id", AggID: []byte(wantAggID), ValueShare: 123})

	if err := ptest.Run(pipeline); err != nil {
		t.Fatalf("pipeline failed: %s", err)
	}
}
