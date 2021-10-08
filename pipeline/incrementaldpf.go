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

	dpfpb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/pipeline/crypto_go_proto"
)

// DefaultElementBitSize is the default element size for generating the DPF keys.
const DefaultElementBitSize = 64

func freeCBytes(cb C.struct_CBytes) {
	C.free(unsafe.Pointer(cb.c))
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

// GenerateKeys generates a pair of DpfKeys for given parameters.
func GenerateKeys(params []*dpfpb.DpfParameters, alpha uint64, betas []uint64) (*dpfpb.DpfKey, *dpfpb.DpfKey, error) {
	cParamsSize := C.int64_t(len(params))
	cParams, cParamPointers, err := createCParams(params)
	defer freeCParams(cParams, cParamPointers)
	if err != nil {
		return nil, nil, err
	}

	betasSize := len(betas)
	cBetasSize := C.int64_t(betasSize)
	var betasPointer *uint64
	if betasSize > 0 {
		betasPointer = &betas[0]
	}

	cKey1 := C.struct_CBytes{}
	cKey2 := C.struct_CBytes{}
	errStr := C.struct_CBytes{}
	status := C.CGenerateKeys(cParams, cParamsSize, C.uint64_t(alpha), (*C.uint64_t)(unsafe.Pointer(betasPointer)), cBetasSize, &cKey1, &cKey2, &errStr)
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
func EvaluateNext64(prefixes []uint64, evalCtx *dpfpb.EvaluationContext) ([]uint64, error) {
	pSize := len(prefixes)
	cPrefixesSize := C.int64_t(pSize)
	var prefixesPointer *uint64
	if pSize > 0 {
		prefixesPointer = &prefixes[0]
	}

	bEvalCtx, err := proto.Marshal(evalCtx)
	if err != nil {
		return nil, err
	}
	cEvalCtx := C.struct_CBytes{c: (*C.char)(C.CBytes(bEvalCtx)), l: C.int(len(bEvalCtx))}
	outExpanded := C.struct_CUInt64Vec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateNext64((*C.uint64_t)(unsafe.Pointer(prefixesPointer)), cPrefixesSize, &cEvalCtx, &outExpanded, &errStr)
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
func EvaluateUntil64(hierarchyLevel int, prefixes []uint64, evalCtx *dpfpb.EvaluationContext) ([]uint64, error) {
	pSize := len(prefixes)
	cPrefixesSize := C.int64_t(pSize)
	var prefixesPointer *uint64
	if pSize > 0 {
		prefixesPointer = &prefixes[0]
	}

	bEvalCtx, err := proto.Marshal(evalCtx)
	if err != nil {
		return nil, err
	}
	cEvalCtx := C.struct_CBytes{c: (*C.char)(C.CBytes(bEvalCtx)), l: C.int(len(bEvalCtx))}
	outExpanded := C.struct_CUInt64Vec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateUntil64(C.int(hierarchyLevel), (*C.uint64_t)(unsafe.Pointer(prefixesPointer)), cPrefixesSize, &cEvalCtx, &outExpanded, &errStr)
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

// CalculateBucketID gets the bucket ID for values in the expanded vectors for certain level of hierarchy.
// If previousLevel = 1, the DPF key has not been evaluated yet:
// http://github.com/google/distributed_point_functions/dpf/distributed_point_function.cc?l=730&rcl=396584858
func CalculateBucketID(params *pb.IncrementalDpfParameters, prefixes *pb.HierarchicalPrefixes, levels []int32, previousLevel int32) ([]uint64, error) {
	if err := CheckExpansionParameters(params, prefixes, levels, previousLevel); err != nil {
		return nil, err
	}

	// For direct expansion, return empty slice to avoid generating extra data.
	// Because in this case, the bucket ID equals the vector index.
	if len(levels) == 1 && previousLevel == -1 {
		return nil, nil
	}

	var prefixBitSize int32
	if len(levels) > 1 {
		prefixBitSize = params.Params[levels[len(levels)-2]].GetLogDomainSize()
	} else {
		prefixBitSize = params.Params[previousLevel].GetLogDomainSize()
	}
	finalBitSize := params.Params[levels[len(levels)-1]].GetLogDomainSize()
	finalPrefixes := prefixes.Prefixes[len(prefixes.Prefixes)-1]

	expansionBits := finalBitSize - prefixBitSize
	expansionSize := uint64(1) << expansionBits
	ids := make([]uint64, uint64(len(finalPrefixes.Prefix))*expansionSize)
	i := uint64(0)
	for _, p := range finalPrefixes.Prefix {
		prefix := p << uint64(expansionBits)
		for j := uint64(0); j < expansionSize; j++ {
			ids[i] = prefix | j
			i++
		}
	}
	return ids, nil
}

func validateLevels(levels []int32) error {
	if len(levels) == 0 {
		return errors.New("expect non-empty levels")
	}
	level := levels[0]
	if level < 0 {
		return fmt.Errorf("expect non-negative level, got %d", level)
	}
	for i := 1; i < len(levels); i++ {
		if levels[i] <= level {
			return fmt.Errorf("expect levels in ascending order, got %v", levels)
		}
		level = levels[i]
	}
	return nil
}

// CheckExpansionParameters checks if the DPF parameters and prefixes are valid for the hierarchical expansion.
func CheckExpansionParameters(params *pb.IncrementalDpfParameters, prefixes *pb.HierarchicalPrefixes, levels []int32, previousLevel int32) error {
	paramsLen := len(params.Params)
	if paramsLen == 0 {
		return errors.New("empty dpf parameters")
	}

	if err := validateLevels(levels); err != nil {
		return err
	}
	if previousLevel < -1 {
		return fmt.Errorf("expect previousLevel >= -1, got %d", previousLevel)
	}
	if previousLevel >= levels[0] {
		return fmt.Errorf("expect previousLevel < levels[0] = %d, got %d", levels[0], previousLevel)
	}

	prefixesLen := len(prefixes.Prefixes)
	if prefixesLen != len(levels) {
		return fmt.Errorf("expansion level size should equal prefixes size %d, got %d", prefixesLen, len(levels))
	}
	if paramsLen < prefixesLen {
		return fmt.Errorf("expect prefixes size <= DPF parameter size %d, got %d", paramsLen, prefixesLen)
	}

	for i, p := range prefixes.Prefixes {
		if i == 0 && previousLevel < 0 && len(p.Prefix) != 0 {
			return fmt.Errorf("prefixes should be empty for the first level expansion, got %s", prefixes.String())
		}

		if !(i == 0 && previousLevel < 0) && len(p.Prefix) == 0 {
			return fmt.Errorf("prefix cannot be empty except for the first level expansion, got %s with previous level %d", prefixes.String(), previousLevel)
		}
	}
	return nil
}
