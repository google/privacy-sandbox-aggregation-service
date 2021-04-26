#ifndef THIRD_PARTY_PRIVACY_SANDBOX_AGGREGATION_PIPELINE_DISTRIBUTED_POINT_FUNCTION_C_BRIDGE_H_
#define THIRD_PARTY_PRIVACY_SANDBOX_AGGREGATION_PIPELINE_DISTRIBUTED_POINT_FUNCTION_C_BRIDGE_H_

#include <stdint.h>

#include "pipeline/cbytes.h"

#ifdef __cplusplus
extern "C" {
#endif

struct CUInt64Vec {
  uint64_t *vec;
  int64_t vec_size;
};

// The following C wrappers for functions in
// go/incremental-dpf-code/distributed_point_function.h will be called in the Go
// code. Callers are responsible for freeing all the memory pointed by
// parameters prefixed with out_* and mutable_*.

// CGenerateKeys wraps GenerateKeys() in C:
// http://google3/dpf/distributed_point_function.h?l=66&rcl=368424076int
// CGenerateKeys(const struct CBytes *params, int64_t params_size,
int CGenerateKeys(const struct CBytes *params, int64_t params_size,
                  uint64_t alpha, const uint64_t *betas, int64_t betas_size,
                  struct CBytes *out_key1, struct CBytes *out_key2,
                  struct CBytes *out_error);

// CCreateEvaluationContext wraps CreateEvaluationContext() in C:
// http://google3/dpf/distributed_point_function.h?l=83&rcl=368424076
int CCreateEvaluationContext(const struct CBytes *params, int64_t params_size,
                             const struct CBytes *key,
                             struct CBytes *out_eval_context,
                             struct CBytes *out_error);
// CEvaluateNext64 wraps EvaluateNext<uint64_t>() in C:
// http://google3/dpf/distributed_point_function.h?l=119&rcl=368424076
int CEvaluateNext64(const uint64_t *prefixes, int64_t prefixes_size,
                    struct CBytes *mutable_context, struct CUInt64Vec *out_vec,
                    struct CBytes *out_error);

// CEvaluateUntil64 wraps EvaluateUntil<uint64_t>() in C:
// http://google3/dpf/distributed_point_function.h?l=123&rcl=369891806
int CEvaluateUntil64(int hierarchy_level, const uint64_t *prefixes,
                     int64_t prefixes_size, struct CBytes *mutable_context,
                     struct CUInt64Vec *out_vec, struct CBytes *out_error);

#ifdef __cplusplus
}
#endif

#endif  // THIRD_PARTY_PRIVACY_SANDBOX_AGGREGATION_PIPELINE_DISTRIBUTED_POINT_FUNCTION_C_BRIDGE_H_
