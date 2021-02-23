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

// Package elgamalencrypttesting contains test functions for homomorphic (ElGamal) encryption.
package elgamalencrypttesting

// #cgo CFLAGS: -g -Wall
// #include <stdbool.h>
// #include <stdlib.h>
// #include "pipeline/elgamal_encrypt_testing_c_bridge.h"
import "C"

import (
	"fmt"
	"unsafe"
)

// GetHashedECPointStrForTesting gets the string-represented ECPoint of the hashed message.
//
// This function is only used for test.
func GetHashedECPointStrForTesting(message string) (string, error) {
	mC := C.CString(message)
	defer C.free(unsafe.Pointer(mC))
	messageC := C.struct_CBytes{c: mC, l: C.int(len(message))}

	resultC := C.struct_CBytes{}
	if !C.CGetHashedECPointStrForTesting(&messageC, &resultC) {
		return "", fmt.Errorf("fail when getting hashed ECPoint for message %s", message)
	}

	result := C.GoStringN(resultC.c, resultC.l)
	defer C.free(unsafe.Pointer(resultC.c))
	return result, nil
}
