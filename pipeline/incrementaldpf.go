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
	"unsafe"

	pb "github.com/google/distributed_point_functions/dpf/distributed_point_function_go_proto"
)

func cBlockToProto(cBlock C.struct_CBlock) *pb.Block {
	return &pb.Block{
		High: uint64(cBlock.high),
		Low:  uint64(cBlock.low),
	}
}

func protoBlockToC(b *pb.Block) C.struct_CBlock {
	return C.struct_CBlock{
		high: C.ulong(b.GetHigh()),
		low:  C.ulong(b.GetLow()),
	}
}

func cDpfParametersToProto(cp C.struct_CDpfParameters) *pb.DpfParameters {
	return &pb.DpfParameters{
		LogDomainSize:  int32(cp.log_domain_size),
		ElementBitsize: int32(cp.element_bitsize),
	}
}

func protoDpfParametersToC(params *pb.DpfParameters) C.struct_CDpfParameters {
	return C.struct_CDpfParameters{
		log_domain_size: C.int(params.GetLogDomainSize()),
		element_bitsize: C.int(params.GetElementBitsize()),
	}
}

func potoCorrectionWordToC(c *pb.CorrectionWord) C.struct_CCorrectionWord {
	return C.struct_CCorrectionWord{
		seed:          protoBlockToC(c.GetSeed()),
		control_left:  C.bool(c.GetControlLeft()),
		control_right: C.bool(c.GetControlRight()),
		output:        protoBlockToC(c.GetOutput()),
	}
}

func cCorrectionWordToProto(cc C.struct_CCorrectionWord) *pb.CorrectionWord {
	return &pb.CorrectionWord{
		Seed:         cBlockToProto(cc.seed),
		ControlLeft:  bool(cc.control_left),
		ControlRight: bool(cc.control_right),
		Output:       cBlockToProto(cc.output),
	}
}

func cDpfKeyToProto(cKey C.struct_CDpfKey) *pb.DpfKey {
	key := &pb.DpfKey{}
	key.Seed = cBlockToProto(cKey.seed)
	cwSize := int(cKey.correction_words_size)
	// Cast the C array pointer into a Go array pointer, then get a slice with certain size. A large Go array size (1 << 30) is used to cover all the data in the C array.
	// See: https://github.com/golang/go/wiki/cgo#turning-c-arrays-into-go-slices
	cwSlice := (*[1 << 30]C.struct_CCorrectionWord)(unsafe.Pointer(cKey.correction_words))[:cwSize:cwSize]
	for i := 0; i < cwSize; i++ {
		key.CorrectionWords = append(key.CorrectionWords, cCorrectionWordToProto(cwSlice[i]))
	}
	key.Party = int32(cKey.party)
	key.LastLevelOutputCorrection = cBlockToProto(cKey.last_level_output_correction)
	return key
}

func protoDpfKeyToC(d *pb.DpfKey) C.struct_CDpfKey {
	cd := C.struct_CDpfKey{}
	cd.seed = protoBlockToC(d.GetSeed())

	cwSize := len(d.GetCorrectionWords())
	cd.correction_words_size = C.long(cwSize)
	cd.correction_words = (*C.struct_CCorrectionWord)(C.malloc(C.sizeof_struct_CCorrectionWord * C.ulong(cwSize)))
	cwSlice := (*[1 << 30]C.struct_CCorrectionWord)(unsafe.Pointer(cd.correction_words))[:cwSize:cwSize]
	for i := 0; i < cwSize; i++ {
		cwSlice[i] = potoCorrectionWordToC(d.GetCorrectionWords()[i])
	}

	cd.party = C.int(d.GetParty())
	cd.last_level_output_correction = protoBlockToC(d.GetLastLevelOutputCorrection())
	return cd
}

func freeCDpfKey(k C.struct_CDpfKey) {
	C.free(unsafe.Pointer(k.correction_words))
}

func cPartialEvaluztionToProto(cpe C.struct_CPartialEvaluation) *pb.PartialEvaluation {
	return &pb.PartialEvaluation{
		Prefix:     cBlockToProto(cpe.prefix),
		Seed:       cBlockToProto(cpe.seed),
		ControlBit: bool(cpe.control_bit),
	}
}

func protoPartialEvaluationToC(p *pb.PartialEvaluation) C.struct_CPartialEvaluation {
	return C.struct_CPartialEvaluation{
		prefix:      protoBlockToC(p.GetPrefix()),
		seed:        protoBlockToC(p.GetSeed()),
		control_bit: C.bool(p.GetControlBit()),
	}
}

func cEvaluationContextToProto(cec C.struct_CEvaluationContext) *pb.EvaluationContext {
	ec := &pb.EvaluationContext{}

	pSize := uint64(cec.parameters_size)
	pSlice := (*[1 << 30]C.struct_CDpfParameters)(unsafe.Pointer(cec.parameters))[:pSize:pSize]
	for i := uint64(0); i < pSize; i++ {
		ec.Parameters = append(ec.Parameters, cDpfParametersToProto(pSlice[i]))
	}

	ec.Key = cDpfKeyToProto(cec.key)
	ec.HierarchyLevel = int32(cec.hierarchy_level)

	peSize := uint64(cec.partial_evaluations_size)
	peSlice := (*[1 << 30]C.struct_CPartialEvaluation)(unsafe.Pointer(cec.partial_evaluations))[:peSize:peSize]
	for i := uint64(0); i < peSize; i++ {
		ec.PartialEvaluations = append(ec.PartialEvaluations, cPartialEvaluztionToProto(peSlice[i]))
	}
	return ec
}

func protoEvaluationContextToC(ctx *pb.EvaluationContext) C.struct_CEvaluationContext {
	ec := C.struct_CEvaluationContext{}

	pSize := len(ctx.GetParameters())
	ec.parameters_size = C.long(pSize)
	ec.parameters = (*C.struct_CDpfParameters)(C.malloc(C.sizeof_struct_CDpfParameters * C.ulong(pSize)))
	pSlice := (*[1 << 30]C.struct_CDpfParameters)(unsafe.Pointer(ec.parameters))[:pSize:pSize]
	for i := 0; i < pSize; i++ {
		pSlice[i] = protoDpfParametersToC(ctx.GetParameters()[i])
	}

	ec.key = protoDpfKeyToC(ctx.GetKey())

	ec.hierarchy_level = C.int(ctx.GetHierarchyLevel())

	peSize := len(ctx.GetPartialEvaluations())
	ec.partial_evaluations_size = C.long(peSize)
	ec.partial_evaluations = (*C.struct_CPartialEvaluation)(C.malloc(C.sizeof_struct_CPartialEvaluation * C.ulong(peSize)))
	peSlice := (*[1 << 30]C.struct_CPartialEvaluation)(unsafe.Pointer(ec.partial_evaluations))[:peSize:peSize]
	for i := 0; i < peSize; i++ {
		peSlice[i] = protoPartialEvaluationToC(ctx.GetPartialEvaluations()[i])
	}

	return ec
}

func freeCEvaluationContext(ec C.struct_CEvaluationContext) {
	C.free(unsafe.Pointer(ec.parameters))
	freeCDpfKey(ec.key)
	C.free(unsafe.Pointer(ec.partial_evaluations))
}

func freeCBytes(cb C.struct_CBytes) {
	C.free(unsafe.Pointer(cb.c))
}

// GenerateKeys generates a pair of DpfKeys for given parameters.
func GenerateKeys(params *pb.DpfParameters, alpha, beta uint64) (key1, key2 *pb.DpfKey, err error) {
	cParams := protoDpfParametersToC(params)
	keyPair := C.struct_CDpfKeyPair{}
	errStr := C.struct_CBytes{}
	defer freeCBytes(errStr)
	if C.CGenerateKeys(&cParams, C.ulong(alpha), C.ulong(beta), &keyPair, &errStr) != 0 {
		err = errors.New(C.GoStringN(errStr.c, errStr.l))
		return
	}
	key1 = cDpfKeyToProto(keyPair.key1)
	key2 = cDpfKeyToProto(keyPair.key2)
	defer freeCDpfKey(keyPair.key1)
	defer freeCDpfKey(keyPair.key2)

	return
}

// CreateEvaluationContext creates the context for expanding the vectors.
func CreateEvaluationContext(params *pb.DpfParameters, key *pb.DpfKey) (*pb.EvaluationContext, error) {
	cParams := protoDpfParametersToC(params)
	cKey := protoDpfKeyToC(key)
	defer freeCDpfKey(cKey)
	outEvalCtx := C.struct_CEvaluationContext{}
	errStr := C.struct_CBytes{}
	defer freeCBytes(errStr)
	if C.CCreateEvaluationContext(&cParams, &cKey, &outEvalCtx, &errStr) != 0 {
		return nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}
	evalCtx := cEvaluationContextToProto(outEvalCtx)
	defer freeCEvaluationContext(outEvalCtx)
	return evalCtx, nil
}

// EvaluateNext64 evaluates the given DPF key in the evaluation context with the specified configuration.
func EvaluateNext64(params *pb.DpfParameters, prefixes []uint64, ctx *pb.EvaluationContext) ([]uint64, error) {
	cParams := protoDpfParametersToC(params)

	pSize := len(prefixes)
	cPrefixesSize := C.ulong(pSize)
	cPrefixes := (*C.ulong)(C.malloc(C.sizeof_ulong * C.ulong(pSize)))
	defer C.free(unsafe.Pointer(cPrefixes))
	pSlice := (*[1 << 30]C.ulong)(unsafe.Pointer(cPrefixes))[:pSize:pSize]
	for i := 0; i < pSize; i++ {
		pSlice[i] = C.ulong(prefixes[i])
	}

	cCtx := protoEvaluationContextToC(ctx)
	outExpanded := C.struct_CUInt64Vec{}
	errStr := C.struct_CBytes{}
	defer freeCBytes(errStr)
	if C.CEvaluateNext64(&cParams, cPrefixes, cPrefixesSize, &cCtx, &outExpanded, &errStr) != 0 {
		return nil, errors.New(C.GoStringN(errStr.c, errStr.l))
	}
	defer freeCEvaluationContext(cCtx)

	vecLen := uint64(outExpanded.vec_size)
	es := (*[1 << 30]C.ulong)(unsafe.Pointer(outExpanded.vec))[:vecLen:vecLen]
	var expanded []uint64
	for i := uint64(0); i < uint64(vecLen); i++ {
		expanded = append(expanded, uint64(es[i]))
	}
	defer C.free(unsafe.Pointer(outExpanded.vec))
	return expanded, nil
}
