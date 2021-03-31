#ifndef THIRD_PARTY_PRIVACY_SANDBOX_AGGREGATION_PIPELINE_DISTRIBUTED_POINT_FUNCTION_C_BRIDGE_H_
#define THIRD_PARTY_PRIVACY_SANDBOX_AGGREGATION_PIPELINE_DISTRIBUTED_POINT_FUNCTION_C_BRIDGE_H_

#include <stdint.h>

#include "pipeline/cbytes.h"

#ifdef __cplusplus
extern "C" {
#endif

// The following structs are used to convert data from the C++ protobuf to Go.
// TODO: Transfer data between languages directly with serialized
// format.
struct CDpfParameters {
  int32_t log_domain_size;
  int32_t element_bitsize;
};

struct CBlock {
  uint64_t high;
  uint64_t low;
};

struct CCorrectionWord {
  struct CBlock seed;
  bool control_left;
  bool control_right;
  struct CBlock output;
};

struct CDpfKey {
  struct CBlock seed;
  struct CCorrectionWord *correction_words;
  int64_t correction_words_size;
  int party;
  struct CBlock last_level_output_correction;
};

struct CPartialEvaluation {
  struct CBlock prefix;
  struct CBlock seed;
  bool control_bit;
};

struct CEvaluationContext {
  struct CDpfParameters *parameters;
  int64_t parameters_size;
  struct CDpfKey key;
  int hierarchy_level;
  struct CPartialEvaluation *partial_evaluations;
  int64_t partial_evaluations_size;
};

struct CUInt64Vec {
  uint64_t *vec;
  int64_t vec_size;
};

struct CDpfKeyPair {
  struct CDpfKey key1;
  struct CDpfKey key2;
};

// CGenerateKeys wraps GenerateKeys() in C:
// http://google3/dpf/distributed_point_function.h?l=66&rcl=363185341
// Caller of this function needs to free the pointers pointed by
// CDpfKey.correction_words in the two keys of out_dpf_keys.
int CGenerateKeys(const struct CDpfParameters *param, uint64_t alpha,
                  uint64_t beta, struct CDpfKeyPair *out_dpf_keys,
                  struct CBytes *out_error);

// CCreateEvaluationContext wraps CreateEvaluationContext() in C:
// http://google3/dpf/distributed_point_function.h?l=83&rcl=363185341
// Caller of this function needs to free the pointers pointed by
// out_eval_context->parameters and out_eval_context->partial_evaluations.
int CCreateEvaluationContext(const struct CDpfParameters *param,
                             const struct CDpfKey *key,
                             struct CEvaluationContext *out_eval_context,
                             struct CBytes *out_error);

// CEvaluateNext64 wraps EvaluateNext<uint64_t>() in C:
// http://google3/dpf/distributed_point_function.h?l=119&rcl=363185341
// Caller of this function needs to free the pointer pointed by out_vec->vec.
int CEvaluateNext64(const struct CDpfParameters *param,
                    const uint64_t *prefixes, uint64_t prefixes_size,
                    struct CEvaluationContext *mutable_context,
                    struct CUInt64Vec *out_vec, struct CBytes *out_error);

#ifdef __cplusplus
}
#endif

#endif  // THIRD_PARTY_PRIVACY_SANDBOX_AGGREGATION_PIPELINE_DISTRIBUTED_POINT_FUNCTION_C_BRIDGE_H_
