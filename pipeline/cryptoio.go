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

// Package cryptoio contains functions for reading and writing private/public keys.
package cryptoio

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"google.golang.org/protobuf/proto"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

const (
	standardPublicKey  = "STANDARD_PUBLIC_KEY"
	standardPrivateKey = "STANDARD_PRIVATE_KEY"

	elgamalPublicKey  = "ELGAMAL_PUBLIC_KEY"
	elgamalPrivateKey = "ELGAMAL_PRIVATE_KEY"
	elgamalSecret     = "ELGAMAL_SECRET"
)

// ReadElGamalSecret is called by the helper servers, which reads the ElGamal secret.
func ReadElGamalSecret(fileDir string) (string, error) {
	bs, err := ioutil.ReadFile(path.Join(fileDir, elgamalSecret))
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// ReadStandardPrivateKey is called by the helper servers, which reads the standard private key.
func ReadStandardPrivateKey(fileDir string) (*pb.StandardPrivateKey, error) {
	bsPriv, err := ioutil.ReadFile(path.Join(fileDir, standardPrivateKey))
	if err != nil {
		return nil, err
	}
	sPriv := &pb.StandardPrivateKey{}
	if err := proto.Unmarshal(bsPriv, sPriv); err != nil {
		return nil, err
	}
	return sPriv, nil
}

// ReadElGamalPrivateKey is called by the helper servers, which reads the ElGamal private key.
func ReadElGamalPrivateKey(fileDir string) (*pb.ElGamalPrivateKey, error) {
	bhPriv, err := ioutil.ReadFile(path.Join(fileDir, elgamalPrivateKey))
	if err != nil {
		return nil, err
	}
	hPriv := &pb.ElGamalPrivateKey{}
	if err := proto.Unmarshal(bhPriv, hPriv); err != nil {
		return nil, err
	}
	return hPriv, nil
}

// ReadStandardPublicKey is called by the browser, which reads the standard encryption public key.
func ReadStandardPublicKey(fileDir string) (*pb.StandardPublicKey, error) {
	bsPub, err := ioutil.ReadFile(path.Join(fileDir, standardPublicKey))
	if err != nil {
		return nil, err
	}
	sPub := &pb.StandardPublicKey{}
	if err := proto.Unmarshal(bsPub, sPub); err != nil {
		return nil, err
	}

	return sPub, nil
}

// ReadElGamalPublicKey is called by the browser and the other helper, which reads the homomorphic encryption public key.
func ReadElGamalPublicKey(fileDir string) (*pb.ElGamalPublicKey, error) {
	bhPub, err := ioutil.ReadFile(path.Join(fileDir, elgamalPublicKey))
	if err != nil {
		return nil, err
	}
	hPub := &pb.ElGamalPublicKey{}
	if err := proto.Unmarshal(bhPub, hPub); err != nil {
		return nil, err
	}
	return hPub, nil
}

// SaveStandardPublicKey is called by the browser, which saves the public standard encryption key.
func SaveStandardPublicKey(fileDir string, sPub *pb.StandardPublicKey) error {
	bsPub, err := proto.Marshal(sPub)
	if err != nil {
		return fmt.Errorf("sPub marshal(%s) failed: %v", sPub.String(), err)
	}
	return ioutil.WriteFile(path.Join(fileDir, standardPublicKey), bsPub, os.ModePerm)
}

// SaveElGamalPublicKey is called by the browser and the other helper, which saves the public ElGamal encryption key into a file.
func SaveElGamalPublicKey(fileDir string, hPub *pb.ElGamalPublicKey) error {
	bhPub, err := proto.Marshal(hPub)
	if err != nil {
		return fmt.Errorf("hPub marshal(%s) failed: %v", hPub.String(), err)
	}
	return ioutil.WriteFile(path.Join(fileDir, elgamalPublicKey), bhPub, os.ModePerm)
}

// CreateKeysAndSecret is called by the helper, which generates all the private/public key pairs and secrets.
//
// The private keys and secret are saved by the helper, and the public keys will be saved by other parties in the aggregation process.
func CreateKeysAndSecret(fileDir string) (*pb.StandardPublicKey, *pb.ElGamalPublicKey, error) {
	// Create/save keys for the standard public-key encryption.
	sPriv, sPub, err := standardencrypt.GenerateStandardKeyPair()
	if err != nil {
		return nil, nil, err
	}

	bsPriv, err := proto.Marshal(sPriv)
	if err != nil {
		return nil, nil, err
	}
	err = ioutil.WriteFile(path.Join(fileDir, standardPrivateKey), bsPriv, os.ModePerm)
	if err != nil {
		return nil, nil, err
	}

	// Create/save keys for the ElGamal encryption.
	hPriv, hPub, err := elgamalencrypt.GenerateElGamalKeyPair()
	if err != nil {
		return nil, nil, err
	}

	bhPriv, err := proto.Marshal(hPriv)
	if err != nil {
		return nil, nil, err
	}
	err = ioutil.WriteFile(path.Join(fileDir, elgamalPrivateKey), bhPriv, os.ModePerm)
	if err != nil {
		return nil, nil, err
	}

	secret, err := elgamalencrypt.GenerateSecret()
	if err != nil {
		return nil, nil, err
	}
	return sPub, hPub, ioutil.WriteFile(path.Join(fileDir, elgamalSecret), []byte(secret), os.ModePerm)
}
