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
// #include "encryption/distributed_point_function_c_bridge.h"
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

// ReachTuple stores the information of each record for the Reach MPC protocol.
type ReachTuple struct {
	C  uint64
	Rf uint64
	R  uint64
	Qf uint64
	Q  uint64
}

// CreateReachUint64TupleDpfParameters generats the DPF parameters for Reach tuples with uint64 elements.
func CreateReachUint64TupleDpfParameters(logDomainSize int32) *dpfpb.DpfParameters {
	param := &dpfpb.DpfParameters{
		LogDomainSize: logDomainSize,
		ValueType: &dpfpb.ValueType{
			Type: &dpfpb.ValueType_Tuple_{
				Tuple: &dpfpb.ValueType_Tuple{
					Elements: []*dpfpb.ValueType{
						{Type: &dpfpb.ValueType_Integer_{Integer: &dpfpb.ValueType_Integer{Bitsize: 64}}},
						{Type: &dpfpb.ValueType_Integer_{Integer: &dpfpb.ValueType_Integer{Bitsize: 64}}},
						{Type: &dpfpb.ValueType_Integer_{Integer: &dpfpb.ValueType_Integer{Bitsize: 64}}},
						{Type: &dpfpb.ValueType_Integer_{Integer: &dpfpb.ValueType_Integer{Bitsize: 64}}},
						{Type: &dpfpb.ValueType_Integer_{Integer: &dpfpb.ValueType_Integer{Bitsize: 64}}},
					},
				},
			},
		},
	}

	// This value needs to be in the range [0, 128], and the default is kDefaultSecurityParameter(40) + log_domain_size.
	// https://github.com/google/distributed_point_functions/blob/master/dpf/distributed_point_function.proto#L104
	if logDomainSize+40 > 128 {
		param.SecurityParameter = 128
	}

	return param
}

func getReachModule() uint64 {
	return uint64(C.GetReachModule())
}

// CreateReachIntModNTupleDpfParameters generats the DPF parameters for Reach tuples with IntModN elements.
func CreateReachIntModNTupleDpfParameters(logDomainSize int32) *dpfpb.DpfParameters {
	reachModule := getReachModule()
	return &dpfpb.DpfParameters{
		LogDomainSize: logDomainSize,
		ValueType: &dpfpb.ValueType{
			Type: &dpfpb.ValueType_Tuple_{
				Tuple: &dpfpb.ValueType_Tuple{
					Elements: []*dpfpb.ValueType{
						{Type: &dpfpb.ValueType_IntModN_{IntModN: &dpfpb.ValueType_IntModN{BaseInteger: &dpfpb.ValueType_Integer{Bitsize: 64}, Modulus: &dpfpb.Value_Integer{Value: &dpfpb.Value_Integer_ValueUint64{ValueUint64: reachModule}}}}},
						{Type: &dpfpb.ValueType_IntModN_{IntModN: &dpfpb.ValueType_IntModN{BaseInteger: &dpfpb.ValueType_Integer{Bitsize: 64}, Modulus: &dpfpb.Value_Integer{Value: &dpfpb.Value_Integer_ValueUint64{ValueUint64: reachModule}}}}},
						{Type: &dpfpb.ValueType_IntModN_{IntModN: &dpfpb.ValueType_IntModN{BaseInteger: &dpfpb.ValueType_Integer{Bitsize: 64}, Modulus: &dpfpb.Value_Integer{Value: &dpfpb.Value_Integer_ValueUint64{ValueUint64: reachModule}}}}},
						{Type: &dpfpb.ValueType_IntModN_{IntModN: &dpfpb.ValueType_IntModN{BaseInteger: &dpfpb.ValueType_Integer{Bitsize: 64}, Modulus: &dpfpb.Value_Integer{Value: &dpfpb.Value_Integer_ValueUint64{ValueUint64: reachModule}}}}},
						{Type: &dpfpb.ValueType_IntModN_{IntModN: &dpfpb.ValueType_IntModN{BaseInteger: &dpfpb.ValueType_Integer{Bitsize: 64}, Modulus: &dpfpb.Value_Integer{Value: &dpfpb.Value_Integer_ValueUint64{ValueUint64: reachModule}}}}},
					},
				},
			},
		},
	}
}

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

func createCReachTuples(tuples []*ReachTuple) *C.struct_CReachTuple {
	len := len(tuples)
	cTuples := (*C.struct_CReachTuple)(C.malloc(C.sizeof_struct_CReachTuple * C.uint64_t(len)))
	pSlice := (*[1 << 30]C.struct_CReachTuple)(unsafe.Pointer(cTuples))[:len:len]
	for i := range tuples {
		pSlice[i] = C.struct_CReachTuple{
			c:  C.uint64_t(tuples[i].C),
			rf: C.uint64_t(tuples[i].Rf),
			r:  C.uint64_t(tuples[i].R),
			qf: C.uint64_t(tuples[i].Qf),
			q:  C.uint64_t(tuples[i].Q),
		}
	}
	return cTuples
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

// GenerateReachTupleKeys generates a pair of DpfKeys for given bucket ID and Reach tuple.
func GenerateReachTupleKeys(params []*dpfpb.DpfParameters, alpha uint128.Uint128, betas []*ReachTuple) (*dpfpb.DpfKey, *dpfpb.DpfKey, error) {
	cParamsSize := C.int64_t(len(params))
	cParams, cParamPointers, err := createCParams(params)
	defer freeCParams(cParams, cParamPointers)
	if err != nil {
		return nil, nil, err
	}

	cAlpha := C.struct_CUInt128{lo: C.uint64_t(alpha.Lo), hi: C.uint64_t(alpha.Hi)}
	cBetasSize := C.int64_t(len(betas))
	betasPointer := createCReachTuples(betas)
	defer C.free(unsafe.Pointer(betasPointer))

	cKey1 := C.struct_CBytes{}
	cKey2 := C.struct_CBytes{}
	errStr := C.struct_CBytes{}
	status := C.CGenerateReachTupleKeys(cParams, cParamsSize, &cAlpha, betasPointer, cBetasSize, &cKey1, &cKey2, &errStr)
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

// CreateReachIntModNTuple creates the tuple with elements representing ReachIntModN type.
func CreateReachIntModNTuple(tuple *ReachTuple) {
	cTuple := C.struct_CReachTuple{
		c:  C.uint64_t(tuple.C),
		rf: C.uint64_t(tuple.Rf),
		r:  C.uint64_t(tuple.R),
		qf: C.uint64_t(tuple.Qf),
		q:  C.uint64_t(tuple.Q),
	}
	C.CCreateReachIntModNTuple(&cTuple)

	tuple.C = uint64(cTuple.c)
	tuple.Rf = uint64(cTuple.rf)
	tuple.R = uint64(cTuple.r)
	tuple.Qf = uint64(cTuple.qf)
	tuple.Q = uint64(cTuple.q)
}

// AddReachIntModNTuple adds the elements in tupleB to those in tupleA.
//
// Both tuples have elements representing ReachIntModN type.
func AddReachIntModNTuple(tupleA *ReachTuple, tupleB *ReachTuple) {
	cTuple := C.struct_CReachTuple{
		c:  C.uint64_t(tupleA.C),
		rf: C.uint64_t(tupleA.Rf),
		r:  C.uint64_t(tupleA.R),
		qf: C.uint64_t(tupleA.Qf),
		q:  C.uint64_t(tupleA.Q),
	}
	C.CAddReachIntModNTuple(&cTuple, &C.struct_CReachTuple{
		c:  C.uint64_t(tupleB.C),
		rf: C.uint64_t(tupleB.Rf),
		r:  C.uint64_t(tupleB.R),
		qf: C.uint64_t(tupleB.Qf),
		q:  C.uint64_t(tupleB.Q),
	})

	tupleA.C = uint64(cTuple.c)
	tupleA.Rf = uint64(cTuple.rf)
	tupleA.R = uint64(cTuple.r)
	tupleA.Qf = uint64(cTuple.qf)
	tupleA.Q = uint64(cTuple.q)
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

// EvaluateUntil64UnsafeDefault is the same with EvaluateUntil64Unsafe but does not need the DpfParameters in the evaluation context.
//
// On the C wrapper creates the default DpfParameters for the given keyBitSize, so less data is copied from Go to C++.
func EvaluateUntil64UnsafeDefault(keyBitSize, hierarchyLevel int, prefixesPtr unsafe.Pointer, prefixesLength int64, evalCtx *dpfpb.EvaluationContext) ([]uint64, error) {
	bEvalCtx, err := proto.Marshal(evalCtx)
	if err != nil {
		return nil, err
	}
	cEvalCtx := C.struct_CBytes{c: (*C.char)(C.CBytes(bEvalCtx)), l: C.int(len(bEvalCtx))}
	outExpanded := C.struct_CUInt64Vec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateUntil64Default(C.int(keyBitSize), C.int(hierarchyLevel), (*C.struct_CUInt128)(prefixesPtr), C.int64_t(prefixesLength), &cEvalCtx, &outExpanded, &errStr)

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

// EvaluateAt64UnsafeDefault is the same with EvaluateAt64Unsafe but does not need the DpfParameters.
//
// On the C wrapper creates the default DpfParameters for the given keyBitSize, so less data is copied from Go to C++.
func EvaluateAt64UnsafeDefault(keyBitSize int, hierarchyLevel int, bukcetsPtr unsafe.Pointer, bukcetsLength int64, dpfKey *dpfpb.DpfKey) ([]uint64, error) {
	bDpfKey, err := proto.Marshal(dpfKey)
	if err != nil {
		return nil, err
	}
	cDpfKey := C.struct_CBytes{c: (*C.char)(C.CBytes(bDpfKey)), l: C.int(len(bDpfKey))}
	cResult := C.struct_CUInt64Vec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateAt64Default(C.int(keyBitSize), &cDpfKey, C.int(hierarchyLevel), (*C.struct_CUInt128)(bukcetsPtr), C.int64_t(bukcetsLength), &cResult, &errStr)

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

// EvaluateReachTuple expands the given DPF key into ReachTuple buckets.
func EvaluateReachTuple(evalCtx *dpfpb.EvaluationContext) ([]*ReachTuple, error) {
	bEvalCtx, err := proto.Marshal(evalCtx)
	if err != nil {
		return nil, err
	}
	cEvalCtx := C.struct_CBytes{c: (*C.char)(C.CBytes(bEvalCtx)), l: C.int(len(bEvalCtx))}
	outExpanded := C.struct_CReachTupleVec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateReachTuple(&cEvalCtx, &outExpanded, &errStr)
	defer freeCBytes(cEvalCtx)
	defer C.free(unsafe.Pointer(outExpanded.vec))
	defer freeCBytes(errStr)
	if status != 0 {
		return nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}

	const maxLen = 1 << 30
	vecLen := uint64(outExpanded.vec_size)
	if vecLen > maxLen {
		return nil, fmt.Errorf("vector length %d should not exceed %d", vecLen, maxLen)
	}
	es := (*[maxLen]C.struct_CReachTuple)(unsafe.Pointer(outExpanded.vec))[:vecLen:vecLen]
	expanded := make([]*ReachTuple, vecLen)
	for i := uint64(0); i < uint64(vecLen); i++ {
		expanded[i] = &ReachTuple{
			C:  uint64(es[i].c),
			Rf: uint64(es[i].rf),
			R:  uint64(es[i].r),
			Qf: uint64(es[i].qf),
			Q:  uint64(es[i].q),
		}
	}
	return expanded, nil
}

// EvaluateReachTupleBetweenLevels expands DPF keys at given range of hierarchies into ReachTuple buckets.
func EvaluateReachTupleBetweenLevels(evalCtx *dpfpb.EvaluationContext, prefixLevel, evalLevel int) ([]*ReachTuple, error) {
	bEvalCtx, err := proto.Marshal(evalCtx)
	if err != nil {
		return nil, err
	}
	cEvalCtx := C.struct_CBytes{c: (*C.char)(C.CBytes(bEvalCtx)), l: C.int(len(bEvalCtx))}
	outExpanded := C.struct_CReachTupleVec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateReachTupleBetweenLevels(&cEvalCtx, C.int(prefixLevel), C.int(evalLevel), &outExpanded, &errStr)
	defer freeCBytes(cEvalCtx)
	defer C.free(unsafe.Pointer(outExpanded.vec))
	defer freeCBytes(errStr)
	if status != 0 {
		return nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}

	const maxLen = 1 << 30
	vecLen := uint64(outExpanded.vec_size)
	if vecLen > maxLen {
		return nil, fmt.Errorf("vector length %d should not exceed %d", vecLen, maxLen)
	}
	es := (*[maxLen]C.struct_CReachTuple)(unsafe.Pointer(outExpanded.vec))[:vecLen:vecLen]
	expanded := make([]*ReachTuple, vecLen)
	for i := uint64(0); i < uint64(vecLen); i++ {
		expanded[i] = &ReachTuple{
			C:  uint64(es[i].c),
			Rf: uint64(es[i].rf),
			R:  uint64(es[i].r),
			Qf: uint64(es[i].qf),
			Q:  uint64(es[i].q),
		}
	}
	return expanded, nil
}

// GetDefaultDPFParameters generates the DPF parameters for creating DPF keys or evaluation context for all possible prefix lengths.
func GetDefaultDPFParameters(keyBitSize int) ([]*dpfpb.DpfParameters, error) {
	if keyBitSize <= 0 {
		return nil, fmt.Errorf("keyBitSize should be positive, got %d", keyBitSize)
	}
	allParams := make([]*dpfpb.DpfParameters, keyBitSize)
	for i := int32(1); i <= int32(keyBitSize); i++ {
		allParams[i-1] = &dpfpb.DpfParameters{
			LogDomainSize: i,
			ValueType: &dpfpb.ValueType{
				Type: &dpfpb.ValueType_Integer_{
					Integer: &dpfpb.ValueType_Integer{
						Bitsize: DefaultElementBitSize,
					},
				},
			},
		}
	}
	return allParams, nil
}

func getTupleDPFParametersFullHierarchy(keyBitSize int) ([]*dpfpb.DpfParameters, error) {
	if keyBitSize <= 0 {
		return nil, fmt.Errorf("keyBitSize should be positive, got %d", keyBitSize)
	}
	allParams := make([]*dpfpb.DpfParameters, keyBitSize)
	for i := int32(1); i <= int32(keyBitSize); i++ {
		allParams[i-1] = CreateReachUint64TupleDpfParameters(i)
	}
	return allParams, nil
}

func getTupleDPFParametersSelectedHierarchy(prefixBitSize, evalBitSize, keyBitSize int) ([]*dpfpb.DpfParameters, error) {
	if evalBitSize <= 0 {
		return nil, fmt.Errorf("evalBitSize should be positive, got %d", evalBitSize)
	}
	if prefixBitSize >= evalBitSize {
		return nil, fmt.Errorf("expect prefixBitSize < evalBitSize, got %d >= %d", prefixBitSize, evalBitSize)
	}
	if evalBitSize > keyBitSize {
		return nil, fmt.Errorf("expect evalBitSize <= keyBitSize, got %d >= %d", evalBitSize, keyBitSize)
	}

	var allParams []*dpfpb.DpfParameters
	if prefixBitSize > 0 {
		allParams = append(allParams, CreateReachUint64TupleDpfParameters(int32(prefixBitSize)))
	}
	allParams = append(allParams, CreateReachUint64TupleDpfParameters(int32(evalBitSize)))
	if evalBitSize < keyBitSize {
		allParams = append(allParams, CreateReachUint64TupleDpfParameters(int32(keyBitSize)))
	}
	return allParams, nil
}

// GetTupleDPFParameters generates the DPF parameters for Reach tuples.
func GetTupleDPFParameters(prefixBitSize, evalBitSize, keyBitSize int, fullHierarchy bool) ([]*dpfpb.DpfParameters, error) {
	if fullHierarchy {
		return getTupleDPFParametersFullHierarchy(keyBitSize)
	}
	return getTupleDPFParametersSelectedHierarchy(prefixBitSize, evalBitSize, keyBitSize)
}
