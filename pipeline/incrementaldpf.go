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

// Package incrementaldpf calls the C++ incremental DPF functions from Go.
package incrementaldpf

// #cgo CFLAGS: -g -Wall
// #include <stdbool.h>
// #include <stdlib.h>
// #include "pipeline/distributed_point_function_c_bridge.h"
import (
	"C"
)

import (
	"errors"
	"fmt"
	"unsafe"

	"google.golang.org/protobuf/proto"
	"lukechampine.com/uint128"

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
)

// DefaultElementBitSize is the default element size for generating the DPF keys.
const DefaultElementBitSize = 64

func freeCBytes(cb C.struct_CBytes) {
	C.free(unsafe.Pointer(cb.c))
}

func createCUInt128s(nums []uint128.Uint128) *C.struct_CUInt128 {
	len := len(nums)
	cNums := (*C.struct_CUInt128)(C.malloc(C.sizeof_struct_CUInt128 * C.uint64_t(len)))
	pSlice := (*[1 << 30]C.struct_CUInt128)(unsafe.Pointer(cNums))[:len:len]
	for i := range nums {
		pSlice[i] = C.struct_CUInt128{lo: C.uint64_t(nums[i].Lo), hi: C.uint64_t(nums[i].Hi)}
	}
	return cNums
}

func createCParams(params []*dpfpb.DpfParameters) (*C.struct_CBytes, []unsafe.Pointer, error) {
	paramsLen := len(params)
	cParamPointers := make([]unsafe.Pointer, paramsLen)
	cParams := (*C.struct_CBytes)(C.malloc(C.sizeof_struct_CBytes * C.uint64_t(paramsLen)))
	pSlice := (*[1 << 30]C.struct_CBytes)(unsafe.Pointer(cParams))[:paramsLen:paramsLen]
	for i, p := range params {
		bParam, err := proto.Marshal(p)
		if err != nil {
			return nil, nil, err
		}
		cParamPointers[i] = C.CBytes(bParam)
		pSlice[i] = C.struct_CBytes{c: (*C.char)(cParamPointers[i]), l: C.int(len(bParam))}
	}
	return cParams, cParamPointers, nil
}

func freeCParams(cParams *C.struct_CBytes, cParamPointers []unsafe.Pointer) {
	for _, p := range cParamPointers {
		C.free(p)
	}
	C.free(unsafe.Pointer(cParams))
}

// CreateCUint128ArrayUnsafe copies the Go 128-bit integers to C.
func CreateCUint128ArrayUnsafe(prefixes []uint128.Uint128) (unsafe.Pointer, int64) {
	return unsafe.Pointer(createCUInt128s(prefixes)), int64(len(prefixes))
}

// FreeUnsafePointer frees the pointers in C.
func FreeUnsafePointer(cPrefixes unsafe.Pointer) {
	C.free(cPrefixes)
}

// GenerateKeys generates a pair of DpfKeys for given parameters.
func GenerateKeys(params []*dpfpb.DpfParameters, alpha uint128.Uint128, betas []uint64) (*dpfpb.DpfKey, *dpfpb.DpfKey, error) {
	cParamsSize := C.int64_t(len(params))
	cParams, cParamPointers, err := createCParams(params)
	defer freeCParams(cParams, cParamPointers)
	if err != nil {
		return nil, nil, err
	}

	cAlpha := C.struct_CUInt128{lo: C.uint64_t(alpha.Lo), hi: C.uint64_t(alpha.Hi)}

	betasSize := len(betas)
	cBetasSize := C.int64_t(betasSize)
	var betasPointer *uint64
	if betasSize > 0 {
		betasPointer = &betas[0]
	}

	cKey1 := C.struct_CBytes{}
	cKey2 := C.struct_CBytes{}
	errStr := C.struct_CBytes{}
	status := C.CGenerateKeys(cParams, cParamsSize, &cAlpha, (*C.uint64_t)(unsafe.Pointer(betasPointer)), cBetasSize, &cKey1, &cKey2, &errStr)
	defer freeCBytes(cKey1)
	defer freeCBytes(cKey2)
	defer freeCBytes(errStr)
	if status != 0 {
		return nil, nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}

	key1 := &dpfpb.DpfKey{}
	if err := proto.Unmarshal(C.GoBytes(unsafe.Pointer(cKey1.c), cKey1.l), key1); err != nil {
		return nil, nil, err
	}

	key2 := &dpfpb.DpfKey{}
	if err := proto.Unmarshal(C.GoBytes(unsafe.Pointer(cKey2.c), cKey2.l), key2); err != nil {
		return nil, nil, err
	}

	return key1, key2, nil
}

// CreateEvaluationContext creates the context for expanding the vectors.
func CreateEvaluationContext(params []*dpfpb.DpfParameters, key *dpfpb.DpfKey) (*dpfpb.EvaluationContext, error) {
	cParamsSize := C.int64_t(len(params))
	cParams, cParamPointers, err := createCParams(params)
	defer freeCParams(cParams, cParamPointers)
	if err != nil {
		return nil, err
	}

	bKey, err := proto.Marshal(key)
	if err != nil {
		return nil, err
	}
	cKey := C.struct_CBytes{c: (*C.char)(C.CBytes(bKey)), l: C.int(len(bKey))}
	defer freeCBytes(cKey)

	cEvalCtx := C.struct_CBytes{}
	errStr := C.struct_CBytes{}
	status := C.CCreateEvaluationContext(cParams, cParamsSize, &cKey, &cEvalCtx, &errStr)
	defer freeCBytes(cEvalCtx)
	defer freeCBytes(errStr)
	if status != 0 {
		return nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}

	evalCtx := &dpfpb.EvaluationContext{}
	if err := proto.Unmarshal(C.GoBytes(unsafe.Pointer(cEvalCtx.c), cEvalCtx.l), evalCtx); err != nil {
		return nil, err
	}
	return evalCtx, nil
}

// EvaluateNext64 evaluates the given DPF key in the evaluation context with the specified configuration.
func EvaluateNext64(prefixes []uint128.Uint128, evalCtx *dpfpb.EvaluationContext) ([]uint64, error) {
	cPrefixesSize := C.int64_t(len(prefixes))
	prefixesPointer := createCUInt128s(prefixes)
	defer C.free(unsafe.Pointer(prefixesPointer))

	bEvalCtx, err := proto.Marshal(evalCtx)
	if err != nil {
		return nil, err
	}
	cEvalCtx := C.struct_CBytes{c: (*C.char)(C.CBytes(bEvalCtx)), l: C.int(len(bEvalCtx))}
	outExpanded := C.struct_CUInt64Vec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateNext64(prefixesPointer, cPrefixesSize, &cEvalCtx, &outExpanded, &errStr)
	defer freeCBytes(cEvalCtx)
	defer C.free(unsafe.Pointer(outExpanded.vec))
	defer freeCBytes(errStr)
	if status != 0 {
		return nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}

	if err := proto.Unmarshal(C.GoBytes(unsafe.Pointer(cEvalCtx.c), cEvalCtx.l), evalCtx); err != nil {
		return nil, err
	}

	const maxLen = 1 << 30
	vecLen := uint64(outExpanded.vec_size)
	if vecLen > maxLen {
		return nil, fmt.Errorf("vector length %d should not exceed %d", vecLen, maxLen)
	}
	es := (*[maxLen]C.uint64_t)(unsafe.Pointer(outExpanded.vec))[:vecLen:vecLen]
	expanded := make([]uint64, vecLen)
	for i := uint64(0); i < uint64(vecLen); i++ {
		expanded[i] = uint64(es[i])
	}
	return expanded, nil
}

// EvaluateUntil64 evaluates the given DPF key in the evaluation context to a certain level of hierarchy.
func EvaluateUntil64(hierarchyLevel int, prefixes []uint128.Uint128, evalCtx *dpfpb.EvaluationContext) ([]uint64, error) {
	cPrefixesSize := C.int64_t(len(prefixes))
	prefixesPointer := createCUInt128s(prefixes)
	defer C.free(unsafe.Pointer(prefixesPointer))

	bEvalCtx, err := proto.Marshal(evalCtx)
	if err != nil {
		return nil, err
	}
	cEvalCtx := C.struct_CBytes{c: (*C.char)(C.CBytes(bEvalCtx)), l: C.int(len(bEvalCtx))}
	outExpanded := C.struct_CUInt64Vec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateUntil64(C.int(hierarchyLevel), prefixesPointer, cPrefixesSize, &cEvalCtx, &outExpanded, &errStr)
	defer freeCBytes(cEvalCtx)
	defer C.free(unsafe.Pointer(outExpanded.vec))
	defer freeCBytes(errStr)
	if status != 0 {
		return nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}

	if err := proto.Unmarshal(C.GoBytes(unsafe.Pointer(cEvalCtx.c), cEvalCtx.l), evalCtx); err != nil {
		return nil, err
	}

	const maxLen = 1 << 30
	vecLen := uint64(outExpanded.vec_size)
	if vecLen > maxLen {
		return nil, fmt.Errorf("vector length %d should not exceed %d", vecLen, maxLen)
	}
	es := (*[maxLen]C.uint64_t)(unsafe.Pointer(outExpanded.vec))[:vecLen:vecLen]
	expanded := make([]uint64, vecLen)
	for i := uint64(0); i < uint64(vecLen); i++ {
		expanded[i] = uint64(es[i])
	}
	return expanded, nil
}

// EvaluateUntil64Unsafe evaluates the given DPF key in the evaluation context to a certain level of hierarchy.
func EvaluateUntil64Unsafe(hierarchyLevel int, prefixesPtr unsafe.Pointer, prefixesLength int64, evalCtx *dpfpb.EvaluationContext) ([]uint64, error) {
	bEvalCtx, err := proto.Marshal(evalCtx)
	if err != nil {
		return nil, err
	}
	cEvalCtx := C.struct_CBytes{c: (*C.char)(C.CBytes(bEvalCtx)), l: C.int(len(bEvalCtx))}
	outExpanded := C.struct_CUInt64Vec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateUntil64(C.int(hierarchyLevel), (*C.struct_CUInt128)(prefixesPtr), C.int64_t(prefixesLength), &cEvalCtx, &outExpanded, &errStr)
	defer freeCBytes(cEvalCtx)
	defer C.free(unsafe.Pointer(outExpanded.vec))
	defer freeCBytes(errStr)
	if status != 0 {
		return nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}

	if err := proto.Unmarshal(C.GoBytes(unsafe.Pointer(cEvalCtx.c), cEvalCtx.l), evalCtx); err != nil {
		return nil, err
	}

	const maxLen = 1 << 30
	vecLen := uint64(outExpanded.vec_size)
	if vecLen > maxLen {
		return nil, fmt.Errorf("vector length %d should not exceed %d", vecLen, maxLen)
	}
	es := (*[maxLen]C.uint64_t)(unsafe.Pointer(outExpanded.vec))[:vecLen:vecLen]
	expanded := make([]uint64, vecLen)
	for i := uint64(0); i < uint64(vecLen); i++ {
		expanded[i] = uint64(es[i])
	}
	return expanded, nil
}

// EvaluateAt64 evaluates the given DPF key on the given evaluationPoints at hierarchyLevel, using a DPF with the given parameters.
func EvaluateAt64(params []*dpfpb.DpfParameters, hierarchyLevel int, evaluationPoints []uint128.Uint128, dpfKey *dpfpb.DpfKey) ([]uint64, error) {
	cParamsSize := C.int64_t(len(params))
	cParams, cParamPointers, err := createCParams(params)
	defer freeCParams(cParams, cParamPointers)
	if err != nil {
		return nil, err
	}

	cEvaluationPointsSize := C.int64_t(len(evaluationPoints))
	cEaluationPointsPointer := createCUInt128s(evaluationPoints)
	defer C.free(unsafe.Pointer(cEaluationPointsPointer))

	bDpfKey, err := proto.Marshal(dpfKey)
	if err != nil {
		return nil, err
	}
	cDpfKey := C.struct_CBytes{c: (*C.char)(C.CBytes(bDpfKey)), l: C.int(len(bDpfKey))}
	cResult := C.struct_CUInt64Vec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateAt64(cParams, cParamsSize, &cDpfKey, C.int(hierarchyLevel), cEaluationPointsPointer, cEvaluationPointsSize, &cResult, &errStr)
	defer C.free(unsafe.Pointer(cResult.vec))
	defer freeCBytes(errStr)
	if status != 0 {
		return nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}

	const maxLen = 1 << 30
	vecLen := uint64(cResult.vec_size)
	if vecLen > maxLen {
		return nil, fmt.Errorf("vector length %d should not exceed %d", vecLen, maxLen)
	}
	es := (*[maxLen]C.uint64_t)(unsafe.Pointer(cResult.vec))[:vecLen:vecLen]
	expanded := make([]uint64, vecLen)
	for i := uint64(0); i < uint64(vecLen); i++ {
		expanded[i] = uint64(es[i])
	}
	return expanded, nil
}

// EvaluateAt64Unsafe evaluates the DPF keys the same with EvaluateAt64, except it takes a unsafe pointer which the user is responsible of freeing after calling it.
func EvaluateAt64Unsafe(params []*dpfpb.DpfParameters, hierarchyLevel int, bukcetsPtr unsafe.Pointer, bukcetsLength int64, dpfKey *dpfpb.DpfKey) ([]uint64, error) {
	cParamsSize := C.int64_t(len(params))
	cParams, cParamPointers, err := createCParams(params)
	defer freeCParams(cParams, cParamPointers)
	if err != nil {
		return nil, err
	}

	bDpfKey, err := proto.Marshal(dpfKey)
	if err != nil {
		return nil, err
	}
	cDpfKey := C.struct_CBytes{c: (*C.char)(C.CBytes(bDpfKey)), l: C.int(len(bDpfKey))}
	cResult := C.struct_CUInt64Vec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateAt64(cParams, cParamsSize, &cDpfKey, C.int(hierarchyLevel), (*C.struct_CUInt128)(bukcetsPtr), C.int64_t(bukcetsLength), &cResult, &errStr)
	defer C.free(unsafe.Pointer(cResult.vec))
	defer freeCBytes(errStr)
	if status != 0 {
		return nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}

	const maxLen = 1 << 30
	vecLen := uint64(cResult.vec_size)
	if vecLen > maxLen {
		return nil, fmt.Errorf("vector length %d should not exceed %d", vecLen, maxLen)
	}
	es := (*[maxLen]C.uint64_t)(unsafe.Pointer(cResult.vec))[:vecLen:vecLen]
	expanded := make([]uint64, vecLen)
	for i := uint64(0); i < uint64(vecLen); i++ {
		expanded[i] = uint64(es[i])
	}
	return expanded, nil
}

// CalculateBucketID gets the bucket ID for values in the expanded vectors for certain level of hierarchy.
// If previousLevel = -1, the DPF key has not been evaluated yet:
// http://github.com/google/distributed_point_functions/dpf/distributed_point_function.cc?l=730&rcl=396584858
func CalculateBucketID(params []*dpfpb.DpfParameters, prefixes []uint128.Uint128, level, previousLevel int32) ([]uint128.Uint128, error) {
	if err := CheckLevels(params, level, previousLevel); err != nil {
		return nil, err
	}

	// For full expansion, return empty slice to avoid generating extra data.
	// Because in this case, the bucket ID equals the vector index.
	if previousLevel == -1 {
		return nil, nil
	}

	expansionBits := params[level].GetLogDomainSize() - params[previousLevel].GetLogDomainSize()
	expansionSize := uint64(1) << expansionBits
	ids := make([]uint128.Uint128, uint64(len(prefixes))*expansionSize)
	i := uint64(0)
	for _, p := range prefixes {
		prefix := p.Lsh(uint(expansionBits))
		for j := uint64(0); j < expansionSize; j++ {
			ids[i] = prefix.Or(uint128.From64(j))
			i++
		}
	}
	return ids, nil
}

// CheckLevels checks if the levels are valid for expanding the DPF keys.
func CheckLevels(params []*dpfpb.DpfParameters, level, previousLevel int32) error {
	paramsLen := len(params)
	if paramsLen == 0 {
		return errors.New("empty dpf parameters")
	}

	if previousLevel < -1 {
		return fmt.Errorf("expect previousLevel >= -1, got %d", previousLevel)
	}
	if previousLevel >= level {
		return fmt.Errorf("expect previousLevel < level %d, got %d", level, previousLevel)
	}

	if paramsLen-1 < int(level) {
		return fmt.Errorf("expect level <= DPF parameter size - 1: %d, got %d", paramsLen-1, level)
	}

	return nil
}

// GetVectorLength calculates the length of expanded vectors.
func GetVectorLength(params []*dpfpb.DpfParameters, prefixes []uint128.Uint128, level, previousLevel int32) (uint64, error) {
	if previousLevel == -1 {
		return uint64(1) << params[level].LogDomainSize, nil
	}

	expansionBits := params[level].GetLogDomainSize() - params[previousLevel].GetLogDomainSize()
	expansionSize := uint64(1) << expansionBits
	return uint64(len(prefixes)) * expansionSize, nil
}
