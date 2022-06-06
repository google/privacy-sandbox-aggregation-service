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

// This binary creates pairs of private and public keys for hybrid encryption.
package main

import (
	"context"
	"flag"

	log "github.com/golang/glog"
	"github.com/google/privacy-sandbox-aggregation-service/encryption/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/shared/utils"
)

var (
	kmsKeyURI         = flag.String("kms_key_uri", "", "Key URI of the GCP KMS service.")
	kmsCredentialFile = flag.String("kms_credential_file", "", "Path of the JSON file that stores the credential information for the KMS service.")
	secretProjectID   = flag.String("secret_project_id", "", "ID of the GCP project that provides the SecretManager service.")
	privateKeyDir     = flag.String("private_key_dir", "", "Output directory for the private keys.")
	keyCount          = flag.Int("key_count", 10, "Count of key pairs to generate.")
	notBefore         = flag.String("not_before", "", "Start valid timestamp for the keys.")
	notAfter          = flag.String("not_after", "", "End valid timestamp for the keys.")
	versionID         = flag.String("version_id", "", "Version of the key pairs.")

	publicKeyInfoFile  = flag.String("public_key_info_file", "", "Output file that contains the public keys and related info.")
	privateKeyInfoFile = flag.String("private_key_info_file", "", "Output file that includes information about how to get the private keys.")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	privKeys, pubInfo, err := cryptoio.GenerateHybridKeyPairs(ctx, *keyCount, *notBefore, *notAfter)
	if err != nil {
		log.Exit(err)
	}

	if *kmsKeyURI == "" {
		log.Warning("non-encrypted private key should be stored only for testing")
	}

	privInfo := make(map[string]*cryptoio.ReadStandardPrivateKeyParams)
	for keyID, key := range privKeys {
		privKeyFile := utils.JoinPath(*privateKeyDir, keyID)
		secretName, err := cryptoio.SaveStandardPrivateKey(ctx, &cryptoio.SaveStandardPrivateKeyParams{
			KMSKeyURI:         *kmsKeyURI,
			KMSCredentialPath: *kmsCredentialFile,
			SecretProjectID:   *secretProjectID,
			SecretID:          keyID,
			FilePath:          privKeyFile,
		}, key)
		if err != nil {
			log.Exit(err)
		}
		privInfo[keyID] = &cryptoio.ReadStandardPrivateKeyParams{
			KMSKeyURI:         *kmsKeyURI,
			KMSCredentialPath: *kmsCredentialFile,
			SecretName:        secretName,
			FilePath:          privKeyFile,
		}
	}

	if err := cryptoio.SavePrivateKeyParamsCollection(ctx, privInfo, *privateKeyInfoFile); err != nil {
		log.Exit(err)
	}
	if err := cryptoio.SavePublicKeyVersions(ctx, map[string][]cryptoio.PublicKeyInfo{*versionID: pubInfo}, *publicKeyInfoFile); err != nil {
		log.Exit(err)
	}
}
