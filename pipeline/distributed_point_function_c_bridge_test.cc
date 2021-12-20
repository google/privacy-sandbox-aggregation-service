#include "pipeline/distributed_point_function_c_bridge.h"

#include <sys/param.h>

#include <cstdint>
#include <cstdlib>
#include <limits>

#include <gmock/gmock.h>
#include <gtest/gtest.h>
#include "absl/random/random.h"
#include "absl/status/status.h"
#include "dpf/distributed_point_function.h"
#include "dpf/distributed_point_function.pb.h"
#include "pipeline/cbytes.h"
#include "pipeline/cbytes_utils.h"

namespace {
using ::convagg::crypto::AllocateCBytes;
using ::convagg::crypto::StrToCBytes;
using ::distributed_point_functions::DpfParameters;
using ::distributed_point_functions::ValueType;

TEST(DistributedPointFunctionBridge, TestKeyGenEval) {
  DpfParameters param0, param1;
  param0.set_log_domain_size(2);
  param0.mutable_value_type()->mutable_integer()->set_bitsize(64);
  param1.set_log_domain_size(4);
  param1.mutable_value_type()->mutable_integer()->set_bitsize(64);

  CBytes b_param0, b_param1;
  ASSERT_TRUE(AllocateCBytes(param0.ByteSizeLong(), &b_param0) &&
              param0.SerializePartialToArray(b_param0.c, b_param0.l));
  ASSERT_TRUE(AllocateCBytes(param1.ByteSizeLong(), &b_param1) &&
              param1.SerializePartialToArray(b_param1.c, b_param1.l));

  CBytes params[2] = {b_param0, b_param1};

  CUInt128 alpha = {.lo = 8, .hi = 0};
  uint64_t betas[2] = {1, 1};
  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateKeys(params, /*params_size=*/2, &alpha, betas,
                          /*betas_size=*/2, &b_key1, &b_key2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  CBytes b_eval_ctx1;
  EXPECT_EQ(CCreateEvaluationContext(params, /*params_size=*/2, &b_key1,
                                     &b_eval_ctx1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CBytes b_eval_ctx2;
  EXPECT_EQ(CCreateEvaluationContext(params, /*params_size=*/2, &b_key2,
                                     &b_eval_ctx2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  CUInt128 *prefixes;
  uint64_t prefixes_size = 0;
  CUInt64Vec vec1;
  EXPECT_EQ(
      CEvaluateNext64(prefixes, prefixes_size, &b_eval_ctx1, &vec1, &error),
      static_cast<int>(absl::StatusCode::kOk));
  CUInt64Vec vec2;
  EXPECT_EQ(
      CEvaluateNext64(prefixes, prefixes_size, &b_eval_ctx2, &vec2, &error),
      static_cast<int>(absl::StatusCode::kOk));

  EXPECT_EQ(vec1.vec_size, int64_t{1} << param0.log_domain_size())
      << "expect vector size " << int64_t{1} << param0.log_domain_size()
      << "got" << vec1.vec_size;

  EXPECT_EQ(vec1.vec_size, vec2.vec_size) << "vec size different";

  for (int i = 0; i < vec1.vec_size; i++) {
    if (i == 2) {
      EXPECT_EQ(vec1.vec[i] + vec2.vec[i], 1) << "failed to recover";
    } else {
      EXPECT_EQ(0, vec1.vec[i] + vec2.vec[i]) << "additional value";
    }
  }

  free(b_param0.c);
  free(b_param1.c);
  free(b_key1.c);
  free(b_key2.c);
  free(b_eval_ctx1.c);
  free(b_eval_ctx2.c);
  free(vec1.vec);
  free(vec2.vec);
}

TEST(DistributedPointFunctionBridge, TestMultiLevelKeyGenEval) {
  DpfParameters param0, param1, param2;
  param0.set_log_domain_size(2);
  param0.mutable_value_type()->mutable_integer()->set_bitsize(64);
  param1.set_log_domain_size(4);
  param1.mutable_value_type()->mutable_integer()->set_bitsize(64);
  param2.set_log_domain_size(5);
  param2.mutable_value_type()->mutable_integer()->set_bitsize(64);

  CBytes b_param0, b_param1, b_param2;
  ASSERT_TRUE(AllocateCBytes(param0.ByteSizeLong(), &b_param0) &&
              param0.SerializePartialToArray(b_param0.c, b_param0.l));
  ASSERT_TRUE(AllocateCBytes(param1.ByteSizeLong(), &b_param1) &&
              param1.SerializePartialToArray(b_param1.c, b_param1.l));
  ASSERT_TRUE(AllocateCBytes(param2.ByteSizeLong(), &b_param2) &&
              param2.SerializePartialToArray(b_param2.c, b_param2.l));

  CBytes params[3] = {b_param0, b_param1, b_param2};

  CUInt128 alpha = {.lo = 16, .hi = 0};
  uint64_t betas[3] = {1, 1, 1};
  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateKeys(params, /*params_size=*/3, &alpha, betas,
                          /*betas_size=*/3, &b_key1, &b_key2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  CBytes b_eval_ctx1;
  EXPECT_EQ(CCreateEvaluationContext(params, /*params_size=*/3, &b_key1,
                                     &b_eval_ctx1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CBytes b_eval_ctx2;
  EXPECT_EQ(CCreateEvaluationContext(params, /*params_size=*/3, &b_key2,
                                     &b_eval_ctx2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  CUInt128 *prefixes0;
  uint64_t prefixes0_size = 0;
  CUInt64Vec vec01;
  EXPECT_EQ(CEvaluateUntil64(0, prefixes0, prefixes0_size, &b_eval_ctx1, &vec01,
                             &error),
            static_cast<int>(absl::StatusCode::kOk));
  CUInt64Vec vec02;
  EXPECT_EQ(CEvaluateUntil64(0, prefixes0, prefixes0_size, &b_eval_ctx2, &vec02,
                             &error),
            static_cast<int>(absl::StatusCode::kOk));

  CUInt128 p0 = {.lo = 0, .hi = 0};
  CUInt128 p1 = {.lo = 2, .hi = 0};
  CUInt128 prefixes1[2] = {p0, p1};
  uint64_t prefixes1_size = 2;
  CUInt64Vec vec11;
  EXPECT_EQ(CEvaluateUntil64(2, prefixes1, prefixes1_size, &b_eval_ctx1, &vec11,
                             &error),
            static_cast<int>(absl::StatusCode::kOk));
  CUInt64Vec vec12;
  EXPECT_EQ(CEvaluateUntil64(2, prefixes1, prefixes1_size, &b_eval_ctx2, &vec12,
                             &error),
            static_cast<int>(absl::StatusCode::kOk));

  uint64_t want_size =
      prefixes1_size *
      (int64_t{1} << (param2.log_domain_size() - param0.log_domain_size()));
  EXPECT_EQ(vec11.vec_size, want_size)
      << "expect vector size " << want_size << "got" << vec11.vec_size;

  EXPECT_EQ(vec11.vec_size, vec12.vec_size) << "vec size different";

  for (int i = 0; i < vec11.vec_size; i++) {
    // When i=8, the index is 16 in the histogram.
    if (i == 8) {
      EXPECT_EQ(vec11.vec[i] + vec12.vec[i], 1) << "failed to recover";
    } else {
      EXPECT_EQ(0, vec11.vec[i] + vec12.vec[i]) << "additional value";
    }
  }

  free(b_param0.c);
  free(b_param1.c);
  free(b_param2.c);
  free(b_key1.c);
  free(b_key2.c);
  free(b_eval_ctx1.c);
  free(b_eval_ctx2.c);
  free(vec01.vec);
  free(vec02.vec);
  free(vec11.vec);
  free(vec12.vec);
}

TEST(DistributedPointFunctionBridge, TestReturnError) {
  DpfParameters param;
  param.mutable_value_type()->mutable_integer()->set_bitsize(-1);
  param.set_log_domain_size(-1);

  CBytes b_params;
  ASSERT_TRUE(StrToCBytes(param.SerializeAsString(), &b_params));
  CBytes params[1] = {b_params};

  CUInt128 alpha = {.lo = 16, .hi = 0};
  uint64_t betas[2] = {1, 1};
  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateKeys(params, /*params_size=*/1, &alpha, betas, 2, &b_key1,
                          &b_key2, &error),
            static_cast<int>(absl::StatusCode::kInvalidArgument));

  std::string want_error = "`log_domain_size` must be non-negative";
  EXPECT_EQ(std::string(error.c, error.l), want_error);
  free(error.c);
  free(b_params.c);
}

TEST(DistributedPointFunctionBridge, TestEvaluateAt64) {
  DpfParameters param;
  param.set_log_domain_size(128);
  param.mutable_value_type()->mutable_integer()->set_bitsize(64);

  CBytes b_param;
  ASSERT_TRUE(AllocateCBytes(param.ByteSizeLong(), &b_param) &&
              param.SerializePartialToArray(b_param.c, b_param.l));

  CUInt128 alpha = {.lo = 8, .hi = 0};
  uint64_t beta = 123;
  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateKeys(&b_param, /*params_size=*/1, &alpha, &beta,
                          /*betas_size=*/1, &b_key1, &b_key2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  constexpr int kNumEvaluationPoints = 4;
  CUInt128 evaluation_points[kNumEvaluationPoints] = {
      alpha,
      {.lo = 0, .hi = 0},
      {.lo = 1, .hi = 0},
      {.lo = std::numeric_limits<uint64_t>::max(),
       .hi = std::numeric_limits<uint64_t>::max()}};

  CUInt64Vec vec1;
  EXPECT_EQ(
      CEvaluateAt64(&b_param, /*params_size=*/1, &b_key1, /*hierarchy_level=*/0,
                    evaluation_points, kNumEvaluationPoints, &vec1, &error),
      static_cast<int>(absl::StatusCode::kOk));
  CUInt64Vec vec2;
  EXPECT_EQ(
      CEvaluateAt64(&b_param, /*params_size=*/1, &b_key2, /*hierarchy_level=*/0,
                    evaluation_points, kNumEvaluationPoints, &vec2, &error),
      static_cast<int>(absl::StatusCode::kOk));

  EXPECT_EQ(vec1.vec[0] + vec2.vec[0], beta);
  for (int i = 1; i < kNumEvaluationPoints; ++i) {
    EXPECT_EQ(vec1.vec[i] + vec2.vec[i], 0);
  }

  free(b_param.c);
  free(b_key1.c);
  free(b_key2.c);
  free(vec1.vec);
  free(vec2.vec);
}

ValueType GetReachUint64TupleValueType() {
  ValueType result;
  ValueType::Tuple *tuple = result.mutable_tuple();
  for (int i = 0; i < 5; i++) {
    tuple->add_elements()->mutable_integer()->set_bitsize(8 * sizeof(uint64_t));
  }
  return result;
}

ValueType GetReachIntModNTupleValueType() {
  ValueType result;
  ValueType::Tuple *tuple = result.mutable_tuple();
  for (int i = 0; i < 5; i++) {
    auto m = tuple->add_elements()->mutable_int_mod_n();
    m->mutable_base_integer()->set_bitsize(8 * sizeof(uint64_t));
    m->mutable_modulus()->set_value_uint64(reach_module);
  }
  return result;
}

TEST(DistributedPointFunctionBridge, TestReachUint64TupleKeyGenEval) {
  DpfParameters params;
  params.set_log_domain_size(18);
  (*params.mutable_value_type()) = GetReachUint64TupleValueType();

  CBytes b_params;
  ASSERT_TRUE(AllocateCBytes(params.ByteSizeLong(), &b_params) &&
              params.SerializePartialToArray(b_params.c, b_params.l));

  CUInt128 alpha = {.lo = 8, .hi = 0};
  CReachTuple beta{};
  beta.c = 1;
  beta.rf = 2;
  beta.r = 3;
  beta.qf = 4;
  beta.q = 5;

  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateReachTupleKeys(&b_params, &alpha, &beta, &b_key1, &b_key2,
                                    &error),
            static_cast<int>(absl::StatusCode::kOk));

  CBytes b_eval_ctx1;
  EXPECT_EQ(
      CCreateEvaluationContext(&b_params, 1, &b_key1, &b_eval_ctx1, &error),
      static_cast<int>(absl::StatusCode::kOk));
  CBytes b_eval_ctx2;
  EXPECT_EQ(
      CCreateEvaluationContext(&b_params, 1, &b_key2, &b_eval_ctx2, &error),
      static_cast<int>(absl::StatusCode::kOk));

  CReachTupleVec vec1;
  EXPECT_EQ(CEvaluateReachTuple(&b_eval_ctx1, &vec1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CReachTupleVec vec2;
  EXPECT_EQ(CEvaluateReachTuple(&b_eval_ctx2, &vec2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  EXPECT_EQ(vec1.vec_size, int64_t{1} << params.log_domain_size())
      << "expect vector size " << int64_t{1} << params.log_domain_size()
      << "got" << vec1.vec_size;

  EXPECT_EQ(vec1.vec_size, vec2.vec_size) << "vec size different";

  for (int i = 0; i < vec1.vec_size; i++) {
    if (i == 8) {
      ASSERT_TRUE(vec1.vec[i].c + vec2.vec[i].c == 1 &&
                  vec1.vec[i].rf + vec2.vec[i].rf == 2 &&
                  vec1.vec[i].r + vec2.vec[i].r == 3 &&
                  vec1.vec[i].qf + vec2.vec[i].qf == 4 &&
                  vec1.vec[i].q + vec2.vec[i].q == 5)
          << "failed to recover";
    } else {
      ASSERT_TRUE(vec1.vec[i].c + vec2.vec[i].c == 0 &&
                  vec1.vec[i].rf + vec2.vec[i].rf == 0 &&
                  vec1.vec[i].r + vec2.vec[i].r == 0 &&
                  vec1.vec[i].qf + vec2.vec[i].qf == 0 &&
                  vec1.vec[i].q + vec2.vec[i].q == 0)
          << "failed to recover";
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

TEST(DistributedPointFunctionBridge, TestReachIntModNTupleKeyGenEval) {
  DpfParameters params;
  params.set_log_domain_size(18);
  (*params.mutable_value_type()) = GetReachIntModNTupleValueType();

  CBytes b_params;
  ASSERT_TRUE(AllocateCBytes(params.ByteSizeLong(), &b_params) &&
              params.SerializePartialToArray(b_params.c, b_params.l));

  CUInt128 alpha = {.lo = 8, .hi = 0};
  CReachTuple beta{};
  beta.c = 1;
  beta.rf = 2;
  beta.r = 3;
  beta.qf = 4;
  beta.q = 5;

  CCreateReachIntModNTuple(&beta);

  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateReachTupleKeys(&b_params, &alpha, &beta, &b_key1, &b_key2,
                                    &error),
            static_cast<int>(absl::StatusCode::kOk));

  CBytes b_eval_ctx1;
  EXPECT_EQ(
      CCreateEvaluationContext(&b_params, 1, &b_key1, &b_eval_ctx1, &error),
      static_cast<int>(absl::StatusCode::kOk));
  CBytes b_eval_ctx2;
  EXPECT_EQ(
      CCreateEvaluationContext(&b_params, 1, &b_key2, &b_eval_ctx2, &error),
      static_cast<int>(absl::StatusCode::kOk));

  CReachTupleVec vec1;
  EXPECT_EQ(CEvaluateReachTuple(&b_eval_ctx1, &vec1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CReachTupleVec vec2;
  EXPECT_EQ(CEvaluateReachTuple(&b_eval_ctx2, &vec2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  EXPECT_EQ(vec1.vec_size, int64_t{1} << params.log_domain_size())
      << "expect vector size " << int64_t{1} << params.log_domain_size()
      << "got" << vec1.vec_size;

  EXPECT_EQ(vec1.vec_size, vec2.vec_size) << "vec size different";

  for (int i = 0; i < vec1.vec_size; i++) {
    CAddReachIntModNTuple(&vec1.vec[i], &vec2.vec[i]);
    if (i == 8) {
      ASSERT_TRUE(vec1.vec[i].c == 1 && vec1.vec[i].rf == 2 &&
                  vec1.vec[i].r == 3 && vec1.vec[i].qf == 4 &&
                  vec1.vec[i].q == 5)
          << "failed to recover";
    } else {
      ASSERT_TRUE(vec1.vec[i].c == 0 && vec1.vec[i].rf == 0 &&
                  vec1.vec[i].r == 0 && vec1.vec[i].qf == 0 &&
                  vec1.vec[i].q == 0)
          << "failed to recover";
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

}  // namespace
