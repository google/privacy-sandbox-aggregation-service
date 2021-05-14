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
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"google.golang.org/protobuf/proto"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/elgamalencrypt"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/ioutils"
	"github.com/google/privacy-sandbox-aggregation-service/pipeline/standardencrypt"
	"github.com/google/tink/go/aead"
	"github.com/google/tink/go/core/registry"
	"github.com/google/tink/go/integration/gcpkms"
	"github.com/google/tink/go/keyset"
	"github.com/google/tink/go/tink"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

// The default file names for stored encryption keys and secret.
const (
	DefaultStandardPublicKey  = "STANDARD_PUBLIC_KEY"
	DefaultStandardPrivateKey = "STANDARD_PRIVATE_KEY"

	DefaultElgamalPublicKey  = "ELGAMAL_PUBLIC_KEY"
	DefaultElgamalPrivateKey = "ELGAMAL_PRIVATE_KEY"
	DefaultElgamalSecret     = "ELGAMAL_SECRET"
)

func getAEADForKMS(keyURI, credentialPath string) (tink.AEAD, error) {
	var (
		gcpclient registry.KMSClient
		err       error
	)
	if credentialPath != "" {
		gcpclient, err = gcpkms.NewClientWithCredentials(keyURI, credentialPath)
	} else {
		gcpclient, err = gcpkms.NewClient(keyURI)
	}
	if err != nil {
		return nil, err
	}
	registry.RegisterKMSClient(gcpclient)

	dek := aead.AES128CTRHMACSHA256KeyTemplate()
	kh, err := keyset.NewHandle(aead.KMSEnvelopeAEADKeyTemplate(keyURI, dek))
	if err != nil {
		return nil, err
	}

	return aead.New(kh)
}

// SaveKMSEncryptedStandardPrivateKey encrypts the standard private key with GCP KMS and saves it into a file.
func SaveKMSEncryptedStandardPrivateKey(keyURI, credentialPath, filePath string, sPriv *pb.StandardPrivateKey) error {
	a, err := getAEADForKMS(keyURI, credentialPath)
	if err != nil {
		return err
	}

	ct, err := a.Encrypt(sPriv.Key, nil)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, ct, os.ModePerm)
}

// ReadKMSDecryptedStandardPrivateKey reads the KMS-encrypted standard private key from a file and decrypts it.
func ReadKMSDecryptedStandardPrivateKey(keyURI, credentialPath, filePath string) (*pb.StandardPrivateKey, error) {
	ct, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	a, err := getAEADForKMS(keyURI, credentialPath)
	if err != nil {
		return nil, err
	}

	bsPriv, err := a.Decrypt(ct, nil)
	if err != nil {
		return nil, err
	}
	return &pb.StandardPrivateKey{Key: bsPriv}, nil
}

// ReadElGamalSecret is called by the helper servers, which reads the ElGamal secret.
func ReadElGamalSecret(filePath string) (string, error) {
	bs, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// ReadStandardPrivateKey is called by the helper servers, which reads the standard private key.
func ReadStandardPrivateKey(filePath string) (*pb.StandardPrivateKey, error) {
	bsPriv, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return &pb.StandardPrivateKey{Key: bsPriv}, nil
}

// ReadElGamalPrivateKey is called by the helper servers, which reads the ElGamal private key.
func ReadElGamalPrivateKey(filePath string) (*pb.ElGamalPrivateKey, error) {
	bhPriv, err := ioutil.ReadFile(filePath)
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
func ReadStandardPublicKey(filePath string) (*pb.StandardPublicKey, error) {
	bsPub, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return &pb.StandardPublicKey{Key: bsPub}, nil
}

// ReadElGamalPublicKey is called by the browser and the other helper, which reads the homomorphic encryption public key.
func ReadElGamalPublicKey(filePath string) (*pb.ElGamalPublicKey, error) {
	bhPub, err := ioutil.ReadFile(filePath)
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
func SaveStandardPublicKey(filePath string, sPub *pb.StandardPublicKey) error {
	return ioutil.WriteFile(filePath, sPub.Key, os.ModePerm)
}

// SaveStandardPrivateKey saves the standard encryption private key into a file.
func SaveStandardPrivateKey(filePath string, sPriv *pb.StandardPrivateKey) error {
	return ioutil.WriteFile(filePath, sPriv.Key, os.ModePerm)
}

// SaveElGamalPublicKey is called by the browser and the other helper, which saves the public ElGamal encryption key into a file.
func SaveElGamalPublicKey(filePath string, hPub *pb.ElGamalPublicKey) error {
	bhPub, err := proto.Marshal(hPub)
	if err != nil {
		return fmt.Errorf("hPub marshal(%s) failed: %v", hPub.String(), err)
	}
	return ioutil.WriteFile(filePath, bhPub, os.ModePerm)
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

	if err := SaveStandardPrivateKey(path.Join(fileDir, DefaultStandardPrivateKey), sPriv); err != nil {
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
	err = ioutil.WriteFile(path.Join(fileDir, DefaultElgamalPrivateKey), bhPriv, os.ModePerm)
	if err != nil {
		return nil, nil, err
	}

	secret, err := elgamalencrypt.GenerateSecret()
	if err != nil {
		return nil, nil, err
	}
	return sPub, hPub, ioutil.WriteFile(path.Join(fileDir, DefaultElgamalSecret), []byte(secret), os.ModePerm)
}

// SaveLines saves the input string slice into a file.
func SaveLines(filename string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	fs, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	buf := bufio.NewWriter(fs)

	for _, line := range lines {
		if _, err := buf.WriteString(line); err != nil {
			return err
		}
		if _, err := buf.Write([]byte{'\n'}); err != nil {
			return err
		}
	}

	if err := buf.Flush(); err != nil {
		return err
	}
	return fs.Close()
}

// SavePrefixes saves prefixes to a file.
//
// The file can be stored locally or in a GCS bucket (prefixed with 'gs://').
func SavePrefixes(ctx context.Context, filename string, prefixes *pb.HierarchicalPrefixes) error {
	bPrefixes, err := proto.Marshal(prefixes)
	if err != nil {
		return fmt.Errorf("prefixes marshal(%s) failed: %v", prefixes.String(), err)
	}
	return ioutils.WriteBytes(ctx, bPrefixes, filename)
}

// SaveDPFParameters saves the DPF parameters into a file.
//
// The file can be stored locally or in a GCS bucket (prefixed with 'gs://').
func SaveDPFParameters(ctx context.Context, filename string, params *pb.IncrementalDpfParameters) error {
	bParams, err := proto.Marshal(params)
	if err != nil {
		return fmt.Errorf("params marshal(%s) failed: %v", params.String(), err)
	}
	return ioutils.WriteBytes(ctx, bParams, filename)
}

// ReadPrefixes reads the prefixes from a file.
//
// The file can be stored locally or in a GCS bucket (prefixed with 'gs://').
func ReadPrefixes(ctx context.Context, filename string) (*pb.HierarchicalPrefixes, error) {
	bPrefixes, err := ioutils.ReadBytes(ctx, filename)
	if err != nil {
		return nil, err
	}
	prefixes := &pb.HierarchicalPrefixes{}
	if err := proto.Unmarshal(bPrefixes, prefixes); err != nil {
		return nil, err
	}
	return prefixes, nil
}

// ReadDPFParameters reads the DPF parameters from a file.
//
// The file can be stored locally or in a GCS bucket (prefixed with 'gs://').
func ReadDPFParameters(ctx context.Context, filename string) (*pb.IncrementalDpfParameters, error) {
	bParams, err := ioutils.ReadBytes(ctx, filename)
	if err != nil {
		return nil, err
	}
	params := &pb.IncrementalDpfParameters{}
	if err := proto.Unmarshal(bParams, params); err != nil {
		return nil, err
	}
	return params, nil
}
