#include "pipeline/distributed_point_function_c_bridge.h"

#include <sys/param.h>

#include <cstdlib>

#include <gmock/gmock.h>
#include <gtest/gtest.h>
#include "absl/status/status.h"

namespace {
TEST(DistributedPointFunctionBridge, TestKeyGenEval) {
  CDpfParameters params;
  params.log_domain_size = 8;
  params.element_bitsize = 64;

  uint64_t alpha = 0;
  uint64_t beta = 1;
  CDpfKeyPair key_pair;
  CBytes error;
  EXPECT_EQ(CGenerateKeys(&params, alpha, beta, &key_pair, &error),
            static_cast<int>(absl::StatusCode::kOk));

  CEvaluationContext eval_ctx1;
  EXPECT_EQ(
      CCreateEvaluationContext(&params, &key_pair.key1, &eval_ctx1, &error),
      static_cast<int>(absl::StatusCode::kOk));
  CEvaluationContext eval_ctx2;
  EXPECT_EQ(
      CCreateEvaluationContext(&params, &key_pair.key2, &eval_ctx2, &error),
      static_cast<int>(absl::StatusCode::kOk));

  uint64_t *prefixes;
  uint64_t prefixes_size = 0;
  CUInt64Vec vec1;
  EXPECT_EQ(CEvaluateNext64(&params, prefixes, prefixes_size, &eval_ctx1, &vec1,
                            &error),
            static_cast<int>(absl::StatusCode::kOk));
  CUInt64Vec vec2;
  EXPECT_EQ(CEvaluateNext64(&params, prefixes, prefixes_size, &eval_ctx2, &vec2,
                            &error),
            static_cast<int>(absl::StatusCode::kOk));

  EXPECT_EQ(vec1.vec_size, int64_t{1} << params.log_domain_size)
      << "expect vector size " << int64_t{1} << params.log_domain_size << "got"
      << vec1.vec_size;

  EXPECT_EQ(vec1.vec_size, vec2.vec_size) << "vec size different";

  for (int i = 0; i < vec1.vec_size; i++) {
    if (i == alpha) {
      EXPECT_EQ(vec1.vec[i] + vec2.vec[i], beta) << "failed to recover";
    } else {
      EXPECT_EQ(0, vec1.vec[i] + vec2.vec[i]) << "additional value";
    }
  }

  free(key_pair.key1.correction_words);
  free(key_pair.key2.correction_words);
  free(eval_ctx1.parameters);
  free(eval_ctx1.key.correction_words);
  free(eval_ctx1.partial_evaluations);
  free(eval_ctx2.parameters);
  free(eval_ctx2.key.correction_words);
  free(eval_ctx2.partial_evaluations);
  free(vec1.vec);
  free(vec2.vec);
}

TEST(DistributedPointFunctionBridge, TestReturnError) {
  CDpfParameters params;
  params.element_bitsize = -1;
  params.log_domain_size = -1;

  uint64_t alpha = 0;
  uint64_t beta = 1;
  CDpfKeyPair key_pair;
  CBytes error;
  EXPECT_EQ(CGenerateKeys(&params, alpha, beta, &key_pair, &error),
            static_cast<int>(absl::StatusCode::kInvalidArgument));

  std::string want_error = "`log_domain_size` must be non-negative";
  EXPECT_EQ(std::string(error.c, error.l), want_error);
  free(error.c);
}
}  // namespace
