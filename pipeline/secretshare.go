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

// Package secretshare contains helper functions for combining the key and value shares.
package secretshare

import (
	"fmt"
)

// XorBytes applies bitwise xor operation for the input bytes a and b, which have the same length.
func XorBytes(a, b []byte) ([]byte, error) {
	n := len(a)
	if n != len(b) {
		return nil, fmt.Errorf("expect the same len(a) and len(b), got %d and %d", n, len(b))
	}
	if n == 0 {
		return nil, fmt.Errorf("empty input bytes")
	}

	dst := make([]byte, n)
	for i := 0; i < n; i++ {
		dst[i] = a[i] ^ b[i]
	}
	return dst, nil
}

// CombineByteShares gets the original conversion key from two key shares.
func CombineByteShares(a, b []byte) ([]byte, error) {
	n := len(a)
	if len(b) != n {
		return nil, fmt.Errorf("byte shares should have the same lengths, got %d != %d", n, len(b))
	}
	if n == 0 {
		return nil, fmt.Errorf("empty input")
	}
	return XorBytes(a, b)
}

// CombineIntShares gets the original conversion value from two value shares.
//
// According to the overflow behavior of uint32 (https://golang.org/ref/spec#Arithmetic_operators),
// the input number can be split into random shares, which can be recovered by summing them up.
func CombineIntShares(a, b uint32) uint32 {
	return a + b
}
