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
	return &dpfpb.DpfParameters{
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

// GenerateReachTupleKeys generates a pair of DpfKeys for given bucket ID and Reach tuple.
func GenerateReachTupleKeys(params *dpfpb.DpfParameters, alpha uint64, beta *ReachTuple) (*dpfpb.DpfKey, *dpfpb.DpfKey, error) {
	cParams, cParamPointers, err := createCParams([]*dpfpb.DpfParameters{params})
	defer freeCParams(cParams, cParamPointers)
	if err != nil {
		return nil, nil, err
	}

	betaTuple := C.struct_CReachTuple{c: C.uint64_t(beta.C), rf: C.uint64_t(beta.Rf), r: C.uint64_t(beta.R), qf: C.uint64_t(beta.Qf), q: C.uint64_t(beta.Q)}

	cKey1 := C.struct_CBytes{}
	cKey2 := C.struct_CBytes{}
	errStr := C.struct_CBytes{}
	status := C.CGenerateReachTupleKeys(cParams, C.uint64_t(alpha), &betaTuple, &cKey1, &cKey2, &errStr)
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

// CalculateBucketID gets the bucket ID for values in the expanded vectors for certain level of hierarchy.
//
// 'prefixLevel' is the level of hierarchy that the prefixes[prefixLevel+1] are applied to filter results;
// 'expandLevel' is the next level that the filtered results are further expanded.
func CalculateBucketID(params *pb.IncrementalDpfParameters, prefixes *pb.HierarchicalPrefixes, prefixLevel, expandLevel int) ([]uint64, error) {
	if err := CheckExpansionParameters(params, prefixes); err != nil {
		return nil, err
	}

	paramsLen := len(params.Params)
	// For direct expansion, return empty slice to avoid generating extra data.
	// Because in this case, the bucket ID equals the vector index.
	if paramsLen == 1 {
		return nil, nil
	}

	if prefixLevel < 0 || prefixLevel > expandLevel || expandLevel >= paramsLen {
		return nil, fmt.Errorf("expect 0 <= prefixLevel <= expandLevel < %d, got prefixLevel=%d, expandLevel=%d", paramsLen, prefixLevel, expandLevel)
	}

	expansionBits := params.Params[expandLevel].GetLogDomainSize() - params.Params[prefixLevel].GetLogDomainSize()
	expansionSize := uint64(1) << expansionBits
	ids := make([]uint64, uint64(len(prefixes.Prefixes[prefixLevel+1].Prefix))*expansionSize)
	i := uint64(0)
	for _, p := range prefixes.Prefixes[prefixLevel+1].Prefix {
		prefix := p << uint64(expansionBits)
		for j := uint64(0); j < expansionSize; j++ {
			ids[i] = prefix | j
			i++
		}
	}
	return ids, nil
}

// CheckExpansionParameters checks if the DPF parameters and prefixes are valid for the hierarchical expansion.
func CheckExpansionParameters(params *pb.IncrementalDpfParameters, prefixes *pb.HierarchicalPrefixes) error {
	paramsLen := len(params.Params)
	if paramsLen == 0 {
		return errors.New("empty dpf parameters")
	}

	prefixesLen := len(prefixes.Prefixes)
	if paramsLen != prefixesLen {
		return fmt.Errorf("dpf parameter size should equal prefixes size %d, got %d", prefixesLen, paramsLen)
	}

	if len(prefixes.Prefixes[0].Prefix) != 0 {
		return fmt.Errorf("prefixes should be empty for the first level expansion, got %s", prefixes.Prefixes[0].String())
	}

	for i, p := range prefixes.Prefixes {
		if i > 0 && len(p.Prefix) == 0 {
			return fmt.Errorf("prefix cannot be empty except for the top level expansion")
		}
	}
	return nil
}
