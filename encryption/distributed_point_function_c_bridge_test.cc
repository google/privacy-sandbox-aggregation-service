#include "encryption/distributed_point_function_c_bridge.h"

#include <sys/param.h>

#include <cstddef>
#include <cstdint>
#include <cstdlib>
#include <limits>
#include <vector>

#include <gmock/gmock.h>
#include <gtest/gtest.h>
#include "absl/numeric/int128.h"
#include "absl/random/random.h"
#include "absl/status/status.h"
#include "dpf/distributed_point_function.h"
#include "dpf/distributed_point_function.pb.h"
#include "encryption/cbytes.h"
#include "encryption/cbytes_utils.h"

namespace {
using ::convagg::crypto::AllocateCBytes;
using ::convagg::crypto::StrToCBytes;
using ::distributed_point_functions::DistributedPointFunction;
using ::distributed_point_functions::DpfKey;
using ::distributed_point_functions::DpfParameters;
using ::distributed_point_functions::EvaluationContext;
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

std::vector<DpfParameters> GetDefaultDpfParameters(int key_bit_size) {
  std::vector<DpfParameters> parameters(key_bit_size);
  for (int i = 0; i < key_bit_size; i++) {
    parameters[i] = DpfParameters();
    parameters[i].set_log_domain_size(i + 1);
    parameters[i].mutable_value_type()->mutable_integer()->set_bitsize(
        default_element_bit_size);
  }
  return parameters;
}

TEST(DistributedPointFunctionBridge, TestEvaluateAt64Default) {
  constexpr int kKeyBitSize = 128;
  std::vector<DpfParameters> params = GetDefaultDpfParameters(kKeyBitSize);

  CBytes b_params[kKeyBitSize];
  for (int i = 0; i < kKeyBitSize; i++) {
    ASSERT_TRUE(
        AllocateCBytes(params[i].ByteSizeLong(), &b_params[i]) &&
        params[i].SerializePartialToArray(b_params[i].c, b_params[i].l));
  }

  CUInt128 alpha = {.lo = 8, .hi = 0};
  uint64_t betas[kKeyBitSize];
  for (int i = 0; i < kKeyBitSize; i++) {
    betas[i] = 123;
  }
  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateKeys(b_params, kKeyBitSize, &alpha, betas,
                          /*betas_size=*/kKeyBitSize, &b_key1, &b_key2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  constexpr int kNumEvaluationPoints = 4;
  CUInt128 evaluation_points[kNumEvaluationPoints] = {
      alpha,
      {.lo = 0, .hi = 0},
      {.lo = 1, .hi = 0},
      {.lo = std::numeric_limits<uint64_t>::max(),
       .hi = std::numeric_limits<uint64_t>::max()}};

  CUInt64Vec vec1;
  EXPECT_EQ(CEvaluateAt64Default(
                kKeyBitSize, &b_key1, /*hierarchy_level=*/kKeyBitSize - 1,
                evaluation_points, kNumEvaluationPoints, &vec1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CUInt64Vec vec2;
  EXPECT_EQ(CEvaluateAt64Default(
                kKeyBitSize, &b_key2, /*hierarchy_level=*/kKeyBitSize - 1,
                evaluation_points, kNumEvaluationPoints, &vec2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  EXPECT_EQ(vec1.vec[0] + vec2.vec[0], betas[kKeyBitSize - 1]);
  for (int i = 1; i < kNumEvaluationPoints; ++i) {
    EXPECT_EQ(vec1.vec[i] + vec2.vec[i], 0);
  }

  for (auto &p : b_params) {
    free(p.c);
  }
  free(b_key1.c);
  free(b_key2.c);
  free(vec1.vec);
  free(vec2.vec);
}

TEST(DistributedPointFunctionBridge, TestEvaluateUntil64Default) {
  constexpr int kKeyBitSize = 4;

  std::vector<DpfParameters> params = GetDefaultDpfParameters(kKeyBitSize);
  absl::StatusOr<std::unique_ptr<DistributedPointFunction>> dpf =
      DistributedPointFunction::CreateIncremental(std::move(params));
  DCHECK(dpf.ok());

  absl::uint128 alpha = absl::MakeUint128(0, 1);
  std::vector<absl::uint128> betas(kKeyBitSize, absl::MakeUint128(0, 123));
  absl::StatusOr<std::pair<DpfKey, DpfKey>> keys =
      (*dpf)->GenerateKeysIncremental(alpha, betas);
  DCHECK(keys.ok());

  EvaluationContext eval_ctx1, eval_ctx2;
  eval_ctx1.set_previous_hierarchy_level(-1);
  *eval_ctx1.mutable_key() = keys->first;
  eval_ctx2.set_previous_hierarchy_level(-1);
  *eval_ctx2.mutable_key() = keys->second;

  CBytes b_eval_ctx1, b_eval_ctx2, error;
  ASSERT_TRUE(AllocateCBytes(eval_ctx1.ByteSizeLong(), &b_eval_ctx1) &&
              eval_ctx1.SerializePartialToArray(b_eval_ctx1.c, b_eval_ctx1.l));
  ASSERT_TRUE(AllocateCBytes(eval_ctx2.ByteSizeLong(), &b_eval_ctx2) &&
              eval_ctx2.SerializePartialToArray(b_eval_ctx2.c, b_eval_ctx2.l));

  CUInt128 *prefixes;
  uint64_t prefixes_size = 0;
  CUInt64Vec vec1;

  constexpr int hierarchy_level = 1;
  EXPECT_EQ(CEvaluateUntil64Default(kKeyBitSize, hierarchy_level, prefixes,
                                    prefixes_size, &b_eval_ctx1, &vec1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CUInt64Vec vec2;
  EXPECT_EQ(CEvaluateUntil64Default(kKeyBitSize, hierarchy_level, prefixes,
                                    prefixes_size, &b_eval_ctx2, &vec2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  EXPECT_EQ(vec1.vec[0] + vec2.vec[0],
            absl::Uint128Low64(betas[hierarchy_level]));
  for (int i = 1; i < 1 << hierarchy_level; ++i) {
    EXPECT_EQ(vec1.vec[i] + vec2.vec[i], 0);
  }

  free(b_eval_ctx1.c);
  free(b_eval_ctx2.c);
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
  EXPECT_EQ(CGenerateReachTupleKeys(&b_params, 1, &alpha, &beta, 1, &b_key1,
                                    &b_key2, &error),
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
  EXPECT_EQ(CGenerateReachTupleKeys(&b_params, 1, &alpha, &beta, 1, &b_key1,
                                    &b_key2, &error),
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

TEST(DistributedPointFunctionBridge,
     TestReachIntModNTupleKeyGenEvalFullHierarchy) {
  // Possible hierarchical levels: 0 ~ domain_size-1.
  const int domain_size = 18;
  int prefix_level = 10;  // log_domain_size = 11
  int eval_level = 12;    // log_domain_size = 13

  CBytes params[domain_size];
  for (int i = 0; i < domain_size; i++) {
    int log_domain_size = i + 1;
    DpfParameters param;
    param.set_log_domain_size(log_domain_size);
    (*param.mutable_value_type()) = GetReachIntModNTupleValueType();

    CBytes b_param;
    ASSERT_TRUE(AllocateCBytes(param.ByteSizeLong(), &b_param) &&
                param.SerializePartialToArray(b_param.c, b_param.l));
    params[i] = b_param;
  }

  uint64_t nonzero_index = 24;
  uint64_t end_nonzero_index = nonzero_index >> (domain_size - eval_level - 1);
  CUInt128 alpha = {.lo = nonzero_index, .hi = 0};
  CReachTuple beta{};
  beta.c = 1;
  beta.rf = 2;
  beta.r = 3;
  beta.qf = 4;
  beta.q = 5;
  CReachTuple betas[domain_size];
  for (int i = 0; i < domain_size; i++) {
    betas[i] = beta;
  }

  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateReachTupleKeys(params, domain_size, &alpha, betas,
                                    domain_size, &b_key1, &b_key2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  CBytes b_eval_ctx1;
  EXPECT_EQ(CCreateEvaluationContext(params, domain_size, &b_key1, &b_eval_ctx1,
                                     &error),
            static_cast<int>(absl::StatusCode::kOk));
  CBytes b_eval_ctx2;
  EXPECT_EQ(CCreateEvaluationContext(params, domain_size, &b_key2, &b_eval_ctx2,
                                     &error),
            static_cast<int>(absl::StatusCode::kOk));

  CReachTupleVec vec1;
  EXPECT_EQ(CEvaluateReachTupleBetweenLevels(&b_eval_ctx1, prefix_level,
                                             eval_level, &vec1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CReachTupleVec vec2;
  EXPECT_EQ(CEvaluateReachTupleBetweenLevels(&b_eval_ctx2, prefix_level,
                                             eval_level, &vec2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  int64_t want_size = int64_t{1} << (eval_level - prefix_level);
  EXPECT_EQ(vec1.vec_size, want_size)
      << "expect vector size " << want_size << ", got" << vec1.vec_size;

  EXPECT_EQ(vec1.vec_size, vec2.vec_size) << "vec size different";

  for (int i = 0; i < vec1.vec_size; i++) {
    CAddReachIntModNTuple(&vec1.vec[i], &vec2.vec[i]);
    if (i == end_nonzero_index) {
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

  for (int i = 0; i < domain_size; i++) {
    free(params[i].c);
  }
  free(b_key1.c);
  free(b_key2.c);
  free(b_eval_ctx1.c);
  free(b_eval_ctx2.c);
  free(vec1.vec);
  free(vec2.vec);
}

TEST(DistributedPointFunctionBridge,
     TestReachUint64TupleKeyGenEvalFullHierarchyNonEmptyPrefix) {
  // Possible hierarchical levels: 0 ~ domain_size-1.
  const int kDomainSize = 128;
  const int kPrefixLevel = 100;  // log_domain_size = 101
  const int kEvalLevel = 102;    // log_domain_size = 103

  CBytes params[kDomainSize];
  for (int i = 0; i < kDomainSize; i++) {
    int log_domain_size = i + 1;
    DpfParameters param;
    param.set_log_domain_size(log_domain_size);
    (*param.mutable_value_type()) = GetReachUint64TupleValueType();
    // By default security_parameter = log_domain_size + 40, but it cannot be
    // greater than 128.
    if (log_domain_size + 40 > 128) {
      param.set_security_parameter(128);
    }

    CBytes b_param;
    ASSERT_TRUE(AllocateCBytes(param.ByteSizeLong(), &b_param) &&
                param.SerializePartialToArray(b_param.c, b_param.l));
    params[i] = b_param;
  }

  uint64_t want_nonzero_index = 3;
  absl::uint128 nonzero_bucket = absl::MakeUint128(0, want_nonzero_index) <<=
      (kDomainSize - kEvalLevel - 1);
  CUInt128 alpha = {.lo = absl::Uint128Low64(nonzero_bucket),
                    .hi = absl::Uint128High64(nonzero_bucket)};

  CReachTuple beta{};
  beta.c = 1;
  beta.rf = 2;
  beta.r = 3;
  beta.qf = 4;
  beta.q = 5;
  CReachTuple betas[kDomainSize];
  for (int i = 0; i < kDomainSize; i++) {
    betas[i] = beta;
  }

  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateReachTupleKeys(params, kDomainSize, &alpha, betas,
                                    kDomainSize, &b_key1, &b_key2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  CBytes b_eval_ctx1;
  EXPECT_EQ(CCreateEvaluationContext(params, kDomainSize, &b_key1, &b_eval_ctx1,
                                     &error),
            static_cast<int>(absl::StatusCode::kOk));
  CBytes b_eval_ctx2;
  EXPECT_EQ(CCreateEvaluationContext(params, kDomainSize, &b_key2, &b_eval_ctx2,
                                     &error),
            static_cast<int>(absl::StatusCode::kOk));

  CReachTupleVec vec1;
  EXPECT_EQ(CEvaluateReachTupleBetweenLevels(&b_eval_ctx1, kPrefixLevel,
                                             kEvalLevel, &vec1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CReachTupleVec vec2;
  EXPECT_EQ(CEvaluateReachTupleBetweenLevels(&b_eval_ctx2, kPrefixLevel,
                                             kEvalLevel, &vec2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  int64_t want_size = int64_t{1} << (kEvalLevel - kPrefixLevel);
  EXPECT_EQ(vec1.vec_size, want_size)
      << "expect vector size " << want_size << ", got" << vec1.vec_size;

  EXPECT_EQ(vec1.vec_size, vec2.vec_size) << "vec size different";

  for (int i = 0; i < vec1.vec_size; i++) {
    if (i == want_nonzero_index) {
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

  for (int i = 0; i < kDomainSize; i++) {
    free(params[i].c);
  }
  free(b_key1.c);
  free(b_key2.c);
  free(b_eval_ctx1.c);
  free(b_eval_ctx2.c);
  free(vec1.vec);
  free(vec2.vec);
}

TEST(DistributedPointFunctionBridge,
     TestReachUint64TupleKeyGenEvalFullHierarchyEmptyPrefix) {
  // Possible hierarchical levels: 0 ~ domain_size-1.
  const int kDomainSize = 128;
  const int kPrefixLevel = -1;
  const int kEvalLevel = 1;  // log_domain_size = 2

  CBytes params[kDomainSize];
  for (int i = 0; i < kDomainSize; i++) {
    int log_domain_size = i + 1;
    DpfParameters param;
    param.set_log_domain_size(log_domain_size);
    (*param.mutable_value_type()) = GetReachUint64TupleValueType();
    if (log_domain_size + 40 > 128) {
      param.set_security_parameter(128);
    }

    CBytes b_param;
    ASSERT_TRUE(AllocateCBytes(param.ByteSizeLong(), &b_param) &&
                param.SerializePartialToArray(b_param.c, b_param.l));
    params[i] = b_param;
  }

  uint64_t want_nonzero_index = 3;
  absl::uint128 nonzero_bucket = absl::MakeUint128(0, want_nonzero_index) <<=
      (kDomainSize - kEvalLevel - 1);
  CUInt128 alpha = {.lo = absl::Uint128Low64(nonzero_bucket),
                    .hi = absl::Uint128High64(nonzero_bucket)};
  CReachTuple beta{};
  beta.c = 1;
  beta.rf = 2;
  beta.r = 3;
  beta.qf = 4;
  beta.q = 5;
  CReachTuple betas[kDomainSize];
  for (int i = 0; i < kDomainSize; i++) {
    betas[i] = beta;
  }

  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateReachTupleKeys(params, kDomainSize, &alpha, betas,
                                    kDomainSize, &b_key1, &b_key2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  CBytes b_eval_ctx1;
  EXPECT_EQ(CCreateEvaluationContext(params, kDomainSize, &b_key1, &b_eval_ctx1,
                                     &error),
            static_cast<int>(absl::StatusCode::kOk));
  CBytes b_eval_ctx2;
  EXPECT_EQ(CCreateEvaluationContext(params, kDomainSize, &b_key2, &b_eval_ctx2,
                                     &error),
            static_cast<int>(absl::StatusCode::kOk));

  CReachTupleVec vec1;
  EXPECT_EQ(CEvaluateReachTupleBetweenLevels(&b_eval_ctx1, kPrefixLevel,
                                             kEvalLevel, &vec1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CReachTupleVec vec2;
  EXPECT_EQ(CEvaluateReachTupleBetweenLevels(&b_eval_ctx2, kPrefixLevel,
                                             kEvalLevel, &vec2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  int64_t want_size = int64_t{1} << (kEvalLevel - kPrefixLevel);
  EXPECT_EQ(vec1.vec_size, want_size)
      << "expect vector size " << want_size << ", got" << vec1.vec_size;

  EXPECT_EQ(vec1.vec_size, vec2.vec_size) << "vec size different";

  for (int i = 0; i < vec1.vec_size; i++) {
    if (i == want_nonzero_index) {
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

  for (int i = 0; i < kDomainSize; i++) {
    free(params[i].c);
  }
  free(b_key1.c);
  free(b_key2.c);
  free(b_eval_ctx1.c);
  free(b_eval_ctx2.c);
  free(vec1.vec);
  free(vec2.vec);
}

TEST(DistributedPointFunctionBridge,
     TestReachUint64TupleKeyGenEvalSelectedHierarchy) {
  const int kDomainSize = 128;
  const int kPrefixBitSize = 101;
  const int kEvalBitSize = 103;
  const int kLevelNum = 3;

  std::vector<int> subdomain_sizes{kPrefixBitSize, kEvalBitSize, kDomainSize};
  CBytes params[kLevelNum];
  for (int i = 0; i < kLevelNum; i++) {
    int log_domain_size = subdomain_sizes[i];
    DpfParameters param;
    param.set_log_domain_size(log_domain_size);
    (*param.mutable_value_type()) = GetReachUint64TupleValueType();
    if (log_domain_size + 40 > 128) {
      param.set_security_parameter(128);
    }

    CBytes b_param;
    ASSERT_TRUE(AllocateCBytes(param.ByteSizeLong(), &b_param) &&
                param.SerializePartialToArray(b_param.c, b_param.l));
    params[i] = b_param;
  }

  uint64_t nonzero_index = 100663296;
  uint64_t end_nonzero_index = nonzero_index >> (kDomainSize - kEvalBitSize);
  CUInt128 alpha = {.lo = nonzero_index, .hi = 0};

  CReachTuple beta{};
  beta.c = 1;
  beta.rf = 2;
  beta.r = 3;
  beta.qf = 4;
  beta.q = 5;
  CReachTuple betas[3];
  for (int i = 0; i < kLevelNum; i++) {
    betas[i] = beta;
  }

  CBytes b_key1, b_key2;
  CBytes error;
  EXPECT_EQ(CGenerateReachTupleKeys(params, kLevelNum, &alpha, betas, kLevelNum,
                                    &b_key1, &b_key2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  CBytes b_eval_ctx1;
  EXPECT_EQ(CCreateEvaluationContext(params, kLevelNum, &b_key1, &b_eval_ctx1,
                                     &error),
            static_cast<int>(absl::StatusCode::kOk));
  CBytes b_eval_ctx2;
  EXPECT_EQ(CCreateEvaluationContext(params, kLevelNum, &b_key2, &b_eval_ctx2,
                                     &error),
            static_cast<int>(absl::StatusCode::kOk));

  CReachTupleVec vec1;
  EXPECT_EQ(CEvaluateReachTupleBetweenLevels(&b_eval_ctx1, 0, 1, &vec1, &error),
            static_cast<int>(absl::StatusCode::kOk));
  CReachTupleVec vec2;
  EXPECT_EQ(CEvaluateReachTupleBetweenLevels(&b_eval_ctx2, 0, 1, &vec2, &error),
            static_cast<int>(absl::StatusCode::kOk));

  int64_t want_size = int64_t{1} << (kEvalBitSize - kPrefixBitSize);
  EXPECT_EQ(vec1.vec_size, want_size)
      << "expect vector size " << want_size << ", got" << vec1.vec_size;

  EXPECT_EQ(vec1.vec_size, vec2.vec_size) << "vec size different";

  for (int i = 0; i < vec1.vec_size; i++) {
    if (i == end_nonzero_index) {
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

  for (int i = 0; i < kLevelNum; i++) {
    free(params[i].c);
  }
  free(b_key1.c);
  free(b_key2.c);
  free(b_eval_ctx1.c);
  free(b_eval_ctx2.c);
  free(vec1.vec);
  free(vec2.vec);
}
}  // namespace
