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

	pb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
)

func freeCBytes(cb C.struct_CBytes) {
	C.free(unsafe.Pointer(cb.c))
}

// GenerateKeys generates a pair of DpfKeys for given parameters.
func GenerateKeys(params *pb.DpfParameters, alpha, beta uint64) (*pb.DpfKey, *pb.DpfKey, error) {
	bParams, err := proto.Marshal(params)
	if err != nil {
		return nil, nil, err
	}
	cParams := C.struct_CBytes{c: (*C.char)(C.CBytes(bParams)), l: C.int(len(bParams))}
	defer freeCBytes(cParams)

	cKey1 := C.struct_CBytes{}
	cKey2 := C.struct_CBytes{}
	errStr := C.struct_CBytes{}
	status := C.CGenerateKeys(&cParams, C.uint64_t(alpha), C.uint64_t(beta), &cKey1, &cKey2, &errStr)
	defer freeCBytes(cKey1)
	defer freeCBytes(cKey2)
	defer freeCBytes(errStr)
	if status != 0 {
		return nil, nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}

	key1 := &pb.DpfKey{}
	if err := proto.Unmarshal(C.GoBytes(unsafe.Pointer(cKey1.c), cKey1.l), key1); err != nil {
		return nil, nil, err
	}

	key2 := &pb.DpfKey{}
	if err := proto.Unmarshal(C.GoBytes(unsafe.Pointer(cKey2.c), cKey2.l), key2); err != nil {
		return nil, nil, err
	}

	return key1, key2, nil
}

// CreateEvaluationContext creates the context for expanding the vectors.
func CreateEvaluationContext(params *pb.DpfParameters, key *pb.DpfKey) (*pb.EvaluationContext, error) {
	bParams, err := proto.Marshal(params)
	if err != nil {
		return nil, err
	}
	cParams := C.struct_CBytes{c: (*C.char)(C.CBytes(bParams)), l: C.int(len(bParams))}
	defer freeCBytes(cParams)

	bKey, err := proto.Marshal(key)
	if err != nil {
		return nil, err
	}
	cKey := C.struct_CBytes{c: (*C.char)(C.CBytes(bKey)), l: C.int(len(bKey))}
	defer freeCBytes(cKey)

	cEvalCtx := C.struct_CBytes{}
	errStr := C.struct_CBytes{}
	status := C.CCreateEvaluationContext(&cParams, &cKey, &cEvalCtx, &errStr)
	defer freeCBytes(cEvalCtx)
	defer freeCBytes(errStr)
	if status != 0 {
		return nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}

	evalCtx := &pb.EvaluationContext{}
	if err := proto.Unmarshal(C.GoBytes(unsafe.Pointer(cEvalCtx.c), cEvalCtx.l), evalCtx); err != nil {
		return nil, err
	}
	return evalCtx, nil
}

// EvaluateNext64 evaluates the given DPF key in the evaluation context with the specified configuration.
func EvaluateNext64(params *pb.DpfParameters, prefixes []uint64, evalCtx *pb.EvaluationContext) ([]uint64, error) {
	bParams, err := proto.Marshal(params)
	if err != nil {
		return nil, err
	}
	cParams := C.struct_CBytes{c: (*C.char)(C.CBytes(bParams)), l: C.int(len(bParams))}
	defer freeCBytes(cParams)

	pSize := len(prefixes)
	cPrefixesSize := C.int64_t(pSize)
	var cPrefixes *C.uint64_t
	if pSize > 0 {
		cPrefixes = (*C.uint64_t)(unsafe.Pointer(&prefixes[0]))
	}

	bEvalCtx, err := proto.Marshal(evalCtx)
	if err != nil {
		return nil, err
	}
	cEvalCtx := C.struct_CBytes{c: (*C.char)(C.CBytes(bEvalCtx)), l: C.int(len(bEvalCtx))}
	outExpanded := C.struct_CUInt64Vec{}
	errStr := C.struct_CBytes{}
	status := C.CEvaluateNext64(&cParams, cPrefixes, cPrefixesSize, &cEvalCtx, &outExpanded, &errStr)
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
