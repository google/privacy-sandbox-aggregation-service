#include "pipeline/distributed_point_function_c_bridge.h"

#include <sys/param.h>

#include <cstdlib>

#include <gmock/gmock.h>
#include <gtest/gtest.h>
#include "absl/status/status.h"
#include "pipeline/cbytes.h"
#include "pipeline/cbytes_utils.h"
#include "dpf/distributed_point_function.h"
#include "dpf/distributed_point_function.pb.h"

namespace {
using ::convagg::crypto::StrToCBytes;
using ::distributed_point_functions::DpfParameters;

TEST(DistributedPointFunctionBridge, TestKeyGenEval) {
  DpfParameters params;
  params.set_log_domain_size(8);
  params.set_element_bitsize(64);

  CBytes b_params;
  ASSERT_TRUE(StrToCBytes(params.SerializeAsString(), &b_params));

  uint64_t alpha = 0;
  uint64_t beta = 1;
  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateKeys(&b_params, alpha, beta, &b_key1, &b_key2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  CBytes b_eval_ctx1;
  EXPECT_EQ(CCreateEvaluationContext(&b_params, &b_key1, &b_eval_ctx1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CBytes b_eval_ctx2;
  EXPECT_EQ(CCreateEvaluationContext(&b_params, &b_key2, &b_eval_ctx2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  uint64_t *prefixes;
  uint64_t prefixes_size = 0;
  CUInt64Vec vec1;
  EXPECT_EQ(CEvaluateNext64(&b_params, prefixes, prefixes_size, &b_eval_ctx1,
                            &vec1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CUInt64Vec vec2;
  EXPECT_EQ(CEvaluateNext64(&b_params, prefixes, prefixes_size, &b_eval_ctx2,
                            &vec2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  EXPECT_EQ(vec1.vec_size, int64_t{1} << params.log_domain_size())
      << "expect vector size " << int64_t{1} << params.log_domain_size()
      << "got" << vec1.vec_size;

  EXPECT_EQ(vec1.vec_size, vec2.vec_size) << "vec size different";

  for (int i = 0; i < vec1.vec_size; i++) {
    if (i == alpha) {
      EXPECT_EQ(vec1.vec[i] + vec2.vec[i], beta) << "failed to recover";
    } else {
      EXPECT_EQ(0, vec1.vec[i] + vec2.vec[i]) << "additional value";
    }
  }

  free(b_params.c);
  free(b_key1.c);
  free(b_key2.c);
  free(b_eval_ctx1.c);
  free(b_eval_ctx2.c);
  free(vec1.vec);
  free(vec2.vec);
}

TEST(DistributedPointFunctionBridge, TestReturnError) {
  DpfParameters params;
  params.set_element_bitsize(-1);
  params.set_log_domain_size(-1);

  CBytes b_params;
  ASSERT_TRUE(StrToCBytes(params.SerializeAsString(), &b_params));

  uint64_t alpha = 0;
  uint64_t beta = 1;
  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateKeys(&b_params, alpha, beta, &b_key1, &b_key2, &error),
            static_cast<int>(absl::StatusCode::kInvalidArgument));

  std::string want_error = "`log_domain_size` must be non-negative";
  EXPECT_EQ(std::string(error.c, error.l), want_error);
  free(error.c);
  free(b_params.c);
}
}  // namespace
