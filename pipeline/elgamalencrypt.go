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

// Package elgamalencrypt contains functions for homomorphic (ElGamal) encryption.
package elgamalencrypt

// #cgo CFLAGS: -g -Wall
// #include <stdbool.h>
// #include <stdlib.h>
// #include "pipeline/elgamal_encrypt_c_bridge.h"
import "C"

import (
	"fmt"
	"unsafe"

	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

// GenerateElGamalKeyPair creates a pair of ElGamal private/public keys.
func GenerateElGamalKeyPair() (*pb.ElGamalPrivateKey, *pb.ElGamalPublicKey, error) {
	elgamalKeys := C.struct_CElGamalKeys{}
	if !C.CGenerateElGamalKeyPair(&elgamalKeys) {
		return nil, nil, fmt.Errorf("fail in ElGamal keys generation")
	}

	publicKey := &pb.ElGamalPublicKey{}
	publicKey.G = []byte(C.GoStringN(elgamalKeys.public_key.g.c, elgamalKeys.public_key.g.l))
	publicKey.Y = []byte(C.GoStringN(elgamalKeys.public_key.y.c, elgamalKeys.public_key.y.l))
	defer C.free(unsafe.Pointer(elgamalKeys.public_key.g.c))
	defer C.free(unsafe.Pointer(elgamalKeys.public_key.y.c))

	privateKey := &pb.ElGamalPrivateKey{}
	privateKey.X = []byte(C.GoStringN(elgamalKeys.private_key.x.c, elgamalKeys.private_key.x.l))
	defer C.free(unsafe.Pointer(elgamalKeys.private_key.x.c))

	return privateKey, publicKey, nil
}

// GenerateSecret generates a secret exponent.
func GenerateSecret() (string, error) {
	secretC := C.struct_CBytes{}
	if !C.CGenerateSecret(&secretC) {
		return "", fmt.Errorf("fail in ElGamal secret generation")
	}
	defer C.free(unsafe.Pointer(secretC.c))
	return C.GoStringN(secretC.c, secretC.l), nil
}

// Encrypt encrypts the given message with the given public key.
func Encrypt(message string, publicKey *pb.ElGamalPublicKey) (*pb.ElGamalCiphertext, error) {
	mc := C.CString(message)
	defer C.free(unsafe.Pointer(mc))
	messageC := C.struct_CBytes{c: mc, l: C.int(len(message))}

	gc := C.CString(string(publicKey.G))
	defer C.free(unsafe.Pointer(gc))
	yc := C.CString(string(publicKey.Y))
	defer C.free(unsafe.Pointer(yc))
	publicKeyC := C.struct_CElGamalPublicKey{
		g: C.struct_CBytes{c: gc, l: C.int(len(publicKey.G))},
		y: C.struct_CBytes{c: yc, l: C.int(len(publicKey.Y))},
	}

	ciphertextC := C.struct_CElGamalCiphertext{}
	if !C.CEncrypt(&messageC, &publicKeyC, &ciphertextC) {
		return nil, fmt.Errorf("fail when encrypting message %s", message)
	}

	ciphertext := &pb.ElGamalCiphertext{}
	ciphertext.U = []byte(C.GoStringN(ciphertextC.u.c, ciphertextC.u.l))
	ciphertext.E = []byte(C.GoStringN(ciphertextC.e.c, ciphertextC.e.l))
	defer C.free(unsafe.Pointer(ciphertextC.u.c))
	defer C.free(unsafe.Pointer(ciphertextC.e.c))

	return ciphertext, nil
}

// Decrypt decrypts with the given private key.
func Decrypt(ciphertext *pb.ElGamalCiphertext, privateKey *pb.ElGamalPrivateKey) (string, error) {
	uc := C.CString(string(ciphertext.U))
	defer C.free(unsafe.Pointer(uc))
	ec := C.CString(string(ciphertext.E))
	defer C.free(unsafe.Pointer(ec))

	ciphertextC := C.struct_CElGamalCiphertext{
		u: C.struct_CBytes{c: uc, l: C.int(len(ciphertext.U))},
		e: C.struct_CBytes{c: ec, l: C.int(len(ciphertext.E))},
	}

	xc := C.CString(string(privateKey.X))
	defer C.free(unsafe.Pointer(xc))
	privateKeyC := C.struct_CElGamalPrivateKey{
		x: C.struct_CBytes{c: xc, l: C.int(len(privateKey.X))},
	}

	decryptedC := C.struct_CBytes{}
	if !C.CDecrypt(&ciphertextC, &privateKeyC, &decryptedC) {
		return "", fmt.Errorf("fail when decrypting %s", ciphertext.String())
	}

	decrypted := C.GoStringN(decryptedC.c, decryptedC.l)
	defer C.free(unsafe.Pointer(decryptedC.c))
	return decrypted, nil
}

// ExponentiateOnCiphertext applies the exponential operation to the ciphered message.
func ExponentiateOnCiphertext(ciphertext *pb.ElGamalCiphertext, publicKey *pb.ElGamalPublicKey, exponent string) (*pb.ElGamalCiphertext, error) {
	uc := C.CString(string(ciphertext.U))
	defer C.free(unsafe.Pointer(uc))
	ec := C.CString(string(ciphertext.E))
	defer C.free(unsafe.Pointer(ec))
	ciphertextC := C.struct_CElGamalCiphertext{
		u: C.struct_CBytes{c: uc, l: C.int(len(ciphertext.U))},
		e: C.struct_CBytes{c: ec, l: C.int(len(ciphertext.E))},
	}

	gc := C.CString(string(publicKey.G))
	defer C.free(unsafe.Pointer(gc))
	yc := C.CString(string(publicKey.Y))
	defer C.free(unsafe.Pointer(yc))
	publicKeyC := C.struct_CElGamalPublicKey{
		g: C.struct_CBytes{c: gc, l: C.int(len(publicKey.G))},
		y: C.struct_CBytes{c: yc, l: C.int(len(publicKey.Y))},
	}

	expc := C.CString(exponent)
	defer C.free(unsafe.Pointer(expc))
	exponentC := C.struct_CBytes{c: expc, l: C.int(len(exponent))}

	resultC := C.struct_CElGamalCiphertext{}
	if !C.CExponentiateOnCiphertext(&ciphertextC, &publicKeyC, &exponentC, &resultC) {
		return nil, fmt.Errorf("fail when doing exponentiation on ciphertext %s, with exponent %s and public key %s", ciphertext.String(), exponent, publicKey.String())
	}

	output := &pb.ElGamalCiphertext{}
	output.U = []byte(C.GoStringN(resultC.u.c, resultC.u.l))
	output.E = []byte(C.GoStringN(resultC.e.c, resultC.e.l))
	defer C.free(unsafe.Pointer(resultC.u.c))
	defer C.free(unsafe.Pointer(resultC.e.c))
	return output, nil
}

// ExponentiateOnECPointStr applies the exponential operation to the decrypted message.
func ExponentiateOnECPointStr(value, exponent string) (string, error) {
	vc := C.CString(value)
	defer C.free(unsafe.Pointer(vc))
	valueC := C.struct_CBytes{c: vc, l: C.int(len(value))}

	expc := C.CString(exponent)
	defer C.free(unsafe.Pointer(expc))
	exponentC := C.struct_CBytes{c: expc, l: C.int(len(exponent))}

	resultC := C.struct_CBytes{}
	if !C.CExponentiateOnECPointStr(&valueC, &exponentC, &resultC) {
		return "", fmt.Errorf("fail when doing exponentiation on %s, with exponent %s", value, exponent)
	}

	output := C.GoStringN(resultC.c, resultC.l)
	defer C.free(unsafe.Pointer(resultC.c))
	return output, nil
}
