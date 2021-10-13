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

// Package onepartyconvert converts the raw reports for the one-party aggregation service design.
package onepartyconvert

import (
	"encoding/base64"
	"encoding/json"

	"google.golang.org/protobuf/proto"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/standardencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/report/reporttypes"
	"github.com/google/privacy-sandbox-aggregation-service/utils/utils"

	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

// EncryptReport encrypts an input report with given public keys.
func EncryptReport(report reporttypes.RawReport, keys []cryptoio.PublicKeyInfo, contextInfo []byte, encryptOutput bool) (*pb.EncryptedReport, error) {
	b, err := json.Marshal(&report)
	if err != nil {
		return nil, err
	}

	payload := reporttypes.Payload{
		Operation: "one-party",
		DPFKey:    b,
	}
	bPayload, err := utils.MarshalCBOR(payload)
	if err != nil {
		return nil, err
	}

	keyID, key, err := cryptoio.GetRandomPublicKey(keys)
	if err != nil {
		return nil, err
	}

	if !encryptOutput {
		return &pb.EncryptedReport{
			EncryptedReport: &pb.StandardCiphertext{Data: bPayload},
			ContextInfo:     contextInfo,
			KeyId:           keyID,
		}, nil
	}

	encrypted, err := standardencrypt.Encrypt(bPayload, contextInfo, key)
	if err != nil {
		return nil, err
	}
	return &pb.EncryptedReport{EncryptedReport: encrypted, ContextInfo: contextInfo, KeyId: keyID}, nil
}

// FormatEncryptedReport serializes the EncryptedReport into a string.
func FormatEncryptedReport(encrypted *pb.EncryptedReport) (string, error) {
	bEncrypted, err := proto.Marshal(encrypted)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bEncrypted), nil
}
