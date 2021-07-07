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

// This binary creates a pair of private and public keys for hybrid encryption.
package main

import (
	"context"
	"flag"
	"fmt"

	log "github.com/golang/glog"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/cryptoio"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"
)

var (
	kmsKeyURI         = flag.String("kms_key_uri", "", "Key URI of the GCP KMS service.")
	kmsCredentialFile = flag.String("kms_credential_file", "", "Path of the JSON file that stores the credential information for the KMS service.")
	secretProjectID   = flag.String("secret_project_id", "", "ID of the GCP project that provides the SecretManager service.")
	secretID          = flag.String("secret_id", "", "Secret ID for data stored with SecretManager service.")
	privateKeyFile    = flag.String("private_key_file", "", "Output file path for the private key.")
	publicKeyFile     = flag.String("public_key_file", "", "Output file path for the public key.")
)

func main() {
	flag.Parse()

	priv, pub, err := standardencrypt.GenerateStandardKeyPair()
	if err != nil {
		log.Exit(err)
	}

	ctx := context.Background()
	if err := cryptoio.SaveStandardPublicKey(*publicKeyFile, pub); err != nil {
		log.Exit(err)
	}

	if *kmsKeyURI == "" {
		log.Warning("non-encrypted private key should be stored only for testing")
	}

	name, err := cryptoio.SaveStandardPrivateKey(ctx, &cryptoio.SaveStandardPrivateKeyParams{
		KMSKeyURI:         *kmsKeyURI,
		KMSCredentialPath: *kmsCredentialFile,
		SecretProjectID:   *secretProjectID,
		SecretID:          *secretID,
		FilePath:          *privateKeyFile,
	}, priv)
	if err != nil {
		log.Exit(err)
	}
	// TODO: save the name of the private-key secret, or map this name with the key ID.
	fmt.Printf("Private key saved by SecretManager with name: %s", name)
}
