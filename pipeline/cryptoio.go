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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"google.golang.org/protobuf/proto"
	"lukechampine.com/uint128"
	"github.com/pborman/uuid"
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

	PublicKeysEnv = "AGGPUBLICKEYS"
)

// PublicKeyInfo contains the details of a standard public key.
type PublicKeyInfo struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`
}

// SavePublicKeyVersions saves the standard public keys and corresponding information.
//
// Keys are saved as an environment variable when filePath is not empty; otherwise as a local or GCS file.
func SavePublicKeyVersions(ctx context.Context, keys map[string][]PublicKeyInfo, filePath string) error {
	bKeys, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	if filePath == "" {
		os.Setenv(PublicKeysEnv, base64.StdEncoding.EncodeToString(bKeys))
		return nil
	}
	return ioutils.WriteBytes(ctx, bKeys, filePath)
}

// ReadPublicKeyVersions reads the standard public keys and corresponding information.
//
// When filePath is empty, keys are read from a environment variable; otherwise from a local or GCS file.
func ReadPublicKeyVersions(ctx context.Context, filePath string) (map[string][]PublicKeyInfo, error) {
	var (
		bKeys []byte
		err   error
	)
	if filePath == "" {
		strKeys := os.Getenv(PublicKeysEnv)
		if strKeys == "" {
			return nil, fmt.Errorf("empty environment variable %q for public keys", PublicKeysEnv)
		}
		bKeys, err = base64.StdEncoding.DecodeString(strKeys)
		if err != nil {
			return nil, err
		}
	} else {
		bKeys, err = ioutils.ReadBytes(ctx, filePath)
		if err != nil {
			return nil, err
		}
	}
	keys := make(map[string][]PublicKeyInfo)
	err = json.Unmarshal(bKeys, &keys)
	return keys, err
}

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

// KMSEncryptData encrypts the input data with GCP KMS.
func KMSEncryptData(ctx context.Context, keyURI, credentialPath string, data []byte) ([]byte, error) {
	a, err := getAEADForKMS(keyURI, credentialPath)
	if err != nil {
		return nil, err
	}

	return a.Encrypt(data, nil)
}

// KMSDecryptData decrypts the input data with GCP KMS.
func KMSDecryptData(ctx context.Context, keyURI, credentialPath string, encryptedData []byte) ([]byte, error) {
	a, err := getAEADForKMS(keyURI, credentialPath)
	if err != nil {
		return nil, err
	}

	return a.Decrypt(encryptedData, nil)
}

// ReadElGamalSecret is called by the helper servers, which reads the ElGamal secret.
func ReadElGamalSecret(filePath string) (string, error) {
	bs, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// ReadStandardPrivateKeyParams contains necessary parameters for function ReadStandardPrivateKey.
type ReadStandardPrivateKeyParams struct {
	// KMSKeyURI and KMSCredentialPath are required by Google Key Mangagement service.
	// If KMSKeyURI is empty, the private key is not encrypted with KMS.
	KMSKeyURI, KMSCredentialPath string
	// SecretName is required by Google SecretManager service.
	// If SecretProjectID is empty, the key is stored without SecretManager.
	SecretName string
	// File path of the (encrypted) private key if it's not stored with SecretManager.
	FilePath string
}

// ReadStandardPrivateKey is called by the helper servers, which reads the standard private key.
func ReadStandardPrivateKey(ctx context.Context, params *ReadStandardPrivateKeyParams) (*pb.StandardPrivateKey, error) {
	var (
		data []byte
		err  error
	)
	if params.SecretName != "" {
		data, err = ioutils.ReadSecret(ctx, params.SecretName)
	} else {
		data, err = ioutils.ReadBytes(ctx, params.FilePath)
	}
	if err != nil {
		return nil, err
	}
	if params.KMSKeyURI != "" {
		data, err = KMSDecryptData(ctx, params.KMSKeyURI, params.KMSCredentialPath, data)
	}
	return &pb.StandardPrivateKey{Key: data}, err
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

// SaveStandardPrivateKeyParams contains necessary parameters for function SaveStandardPrivateKey.
type SaveStandardPrivateKeyParams struct {
	// KMSKeyURI and KMSCredentialPath are required by Google Key Mangagement service.
	// If KMSKeyURI is empty, the private key is not encrypted with KMS.
	KMSKeyURI, KMSCredentialPath string
	// SecretProjectID and SecretID are required by Google SecretManager service.
	// If SecretProjectID is empty, the key is stored without SecretManager.
	SecretProjectID, SecretID string
	// File path of the (encrypted) private key if it's not stored with SecretManager.
	FilePath string
}

// SaveStandardPrivateKey saves the standard encryption private key into a file.
//
// When the private key is stored with Google SecretManager, a secret name should be returned.
// The private keys are allowed to be stored without KMS encryption for testing only, otherwise
// they should always be encrypted before storage.
func SaveStandardPrivateKey(ctx context.Context, params *SaveStandardPrivateKeyParams, privateKey *pb.StandardPrivateKey) (string, error) {
	data := privateKey.Key
	var err error
	if params.KMSKeyURI != "" {
		data, err = KMSEncryptData(ctx, params.KMSKeyURI, params.KMSCredentialPath, data)
		if err != nil {
			return "", err
		}
	}
	if params.SecretProjectID != "" {
		return ioutils.SaveSecret(ctx, data, params.SecretProjectID, params.SecretID)
	}
	return "", ioutils.WriteBytes(ctx, data, params.FilePath)
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
func CreateKeysAndSecret(ctx context.Context, fileDir string) (*pb.StandardPublicKey, *pb.ElGamalPublicKey, error) {
	// Create/save keys for the standard public-key encryption.
	sPriv, sPub, err := standardencrypt.GenerateStandardKeyPair()
	if err != nil {
		return nil, nil, err
	}

	if _, err := SaveStandardPrivateKey(ctx, &SaveStandardPrivateKeyParams{FilePath: path.Join(fileDir, DefaultStandardPrivateKey)}, sPriv); err != nil {
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

// SavePrefixes saves prefixes to a file.
//
// The file can be stored locally or in a GCS bucket (prefixed with 'gs://').
func SavePrefixes(ctx context.Context, filename string, prefixes [][]uint128.Uint128) error {
	bPrefixes, err := json.Marshal(prefixes)
	if err != nil {
		return fmt.Errorf("prefixes marshal(%s) failed: %+v", prefixes, err)
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
func ReadPrefixes(ctx context.Context, filename string) ([][]uint128.Uint128, error) {
	bPrefixes, err := ioutils.ReadBytes(ctx, filename)
	if err != nil {
		return nil, err
	}
	prefixes := [][]uint128.Uint128{}
	if err := json.Unmarshal(bPrefixes, &prefixes); err != nil {
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

// SavePrivateKeyParamsCollection saves the information how the private keys are saved.
func SavePrivateKeyParamsCollection(ctx context.Context, idKeys map[string]*ReadStandardPrivateKeyParams, uri string) error {
	b, err := json.Marshal(idKeys)
	if err != nil {
		return err
	}
	return ioutils.WriteBytes(ctx, b, uri)
}

// ReadPrivateKeyParamsCollection reads the information how the private keys can be read.
func ReadPrivateKeyParamsCollection(ctx context.Context, filePath string) (map[string]*ReadStandardPrivateKeyParams, error) {
	b, err := ioutils.ReadBytes(ctx, filePath)
	if err != nil {
		return nil, err
	}
	output := make(map[string]*ReadStandardPrivateKeyParams)
	if err := json.Unmarshal(b, &output); err != nil {
		return nil, err
	}
	return output, nil
}

// ReadPrivateKeyCollection reads the private storage information from a file, and then uses it to read the private keys.
func ReadPrivateKeyCollection(ctx context.Context, filePath string) (map[string]*pb.StandardPrivateKey, error) {
	keyParams, err := ReadPrivateKeyParamsCollection(ctx, filePath)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]*pb.StandardPrivateKey)
	for keyID, params := range keyParams {
		key, err := ReadStandardPrivateKey(ctx, params)
		if err != nil {
			return nil, err
		}
		keys[keyID] = key
	}
	return keys, nil
}

// GenerateHybridKeyPairs generates encryption key pairs with specified valid time window.
func GenerateHybridKeyPairs(ctx context.Context, keyCount int, notBefore, notAfter string) (map[string]*pb.StandardPrivateKey, []PublicKeyInfo, error) {
	privKeys := make(map[string]*pb.StandardPrivateKey)
	var pubInfo []PublicKeyInfo
	for i := 0; i < keyCount; i++ {
		keyID := uuid.New()
		priv, pub, err := standardencrypt.GenerateStandardKeyPair()
		if err != nil {
			return nil, nil, err
		}
		privKeys[keyID] = priv
		pubInfo = append(pubInfo, PublicKeyInfo{
			ID:        keyID,
			Key:       base64.StdEncoding.EncodeToString(pub.Key),
			NotBefore: notBefore,
			NotAfter:  notAfter,
		})
	}
	return privKeys, pubInfo, nil
}
