#include "encryption/distributed_point_function_c_bridge.h"

#include <alloca.h>

#include <cstdint>
#include <cstdlib>
#include <iostream>
#include <memory>
#include <string>
#include <utility>
#include <vector>

#include "absl/numeric/int128.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/escaping.h"
#include "absl/types/span.h"
#include "dpf/distributed_point_function.h"
#include "dpf/distributed_point_function.pb.h"
#include "dpf/int_mod_n.h"
#include "dpf/tuple.h"
#include "encryption/cbytes.h"
#include "encryption/cbytes_utils.h"

using ::convagg::crypto::AllocateCBytes;
using ::convagg::crypto::StrToCBytes;
using ::distributed_point_functions::DistributedPointFunction;
using ::distributed_point_functions::DpfKey;
using ::distributed_point_functions::DpfParameters;
using ::distributed_point_functions::EvaluationContext;
using ::distributed_point_functions::IntModN;
using ::distributed_point_functions::Tuple;
using ::distributed_point_functions::Value;

// ReachIntModN for Reach.
using ReachIntModN = IntModN<uint64_t, reach_module>;

template <class T>
using ReachTuple = Tuple<T, T, T, T, T>;

absl::uint128 ConvertCUInt128(const struct CUInt128* num) {
  return absl::MakeUint128(num->hi, num->lo);
}

absl::StatusOr<std::unique_ptr<DistributedPointFunction>> CreateIncrementalDpf(
    const struct CBytes* params, int64_t params_size) {
  std::vector<DpfParameters> parameters(params_size);
  for (int i = 0; i < params_size; i++) {
    if (!parameters[i].ParseFromArray(params[i].c, params[i].l)) {
      return absl::InvalidArgumentError("failed to parse DpfParameter");
    }
  }
  return DistributedPointFunction::CreateIncremental(std::move(parameters));
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

int CopyDpfKeys(const std::pair<DpfKey, DpfKey>& keys, struct CBytes* out_key1,
                struct CBytes* out_key2, struct CBytes* out_error) {
  if (!AllocateCBytes(keys.first.ByteSizeLong(), out_key1) ||
      !keys.first.SerializeToArray(out_key1->c, out_key1->l)) {
    StrToCBytes("fail to copy DpfKey", out_error);
    return static_cast<int>(absl::StatusCode::kInternal);
  }
  if (!AllocateCBytes(keys.second.ByteSizeLong(), out_key2) ||
      !keys.second.SerializeToArray(out_key2->c, out_key2->l)) {
    StrToCBytes("fail to copy DpfKey", out_error);
    return static_cast<int>(absl::StatusCode::kInternal);
  }

  return static_cast<int>(absl::StatusCode::kOk);
}

int CGenerateKeys(const struct CBytes* params, int64_t params_size,
                  const struct CUInt128* alpha, const uint64_t* betas,
                  int64_t betas_size, struct CBytes* out_key1,
                  struct CBytes* out_key2, struct CBytes* out_error) {
  absl::StatusOr<std::unique_ptr<DistributedPointFunction>> dpf =
      CreateIncrementalDpf(params, params_size);
  if (!dpf.ok()) {
    StrToCBytes(dpf.status().message(), out_error);
    return dpf.status().raw_code();
  }

  std::vector<absl::uint128> betas_128(betas_size);
  for (int i = 0; i < betas_size; i++) {
    betas_128[i] = absl::uint128(betas[i]);
  }

  absl::StatusOr<std::pair<DpfKey, DpfKey>> keys =
      (*dpf)->GenerateKeysIncremental(ConvertCUInt128(alpha), betas_128);
  if (!keys.ok()) {
    StrToCBytes(keys.status().message(), out_error);
    return keys.status().raw_code();
  }

  return CopyDpfKeys(keys.value(), out_key1, out_key2, out_error);
}

absl::StatusOr<bool> UseReachIntOrIntModN(DpfParameters& parameters) {
  if (!parameters.has_value_type() || !parameters.value_type().has_tuple()) {
    return absl::InvalidArgumentError("expect tuple as value type");
  }
  if (parameters.value_type().tuple().elements_size() != 5) {
    return absl::InvalidArgumentError("expect 5 elements for the Reach tuple");
  }
  int count_int = 0;
  int count_intmodn = 0;

  for (int i = 0; i < 5; i++) {
    if (parameters.value_type().tuple().elements(i).has_integer())
      count_int++;
    else if (parameters.value_type().tuple().elements(i).has_int_mod_n())
      count_intmodn++;
    else
      return absl::InvalidArgumentError(
          "expect int or IntModN elements for the Reach tuple");
  }
  if (count_int == 5)
    return true;
  else if (count_intmodn == 5)
    return false;
  else
    return absl::InvalidArgumentError(
        "expect same element types for the Reach tuple");
}

int CGenerateReachTupleKeys(const struct CBytes* params, int64_t params_size,
                            const struct CUInt128* alpha,
                            const CReachTuple* betas, int64_t betas_size,
                            struct CBytes* out_key1, struct CBytes* out_key2,
                            struct CBytes* out_error) {
  std::vector<DpfParameters> parameters(params_size);
  for (int i = 0; i < params_size; i++) {
    if (!parameters[i].ParseFromArray(params[i].c, params[i].l)) {
      StrToCBytes("failed to parse DpfParameter", out_error);
      return static_cast<int>(absl::StatusCode::kInvalidArgument);
    }
  }

  absl::StatusOr<bool> use_int = UseReachIntOrIntModN(parameters[0]);
  if (!use_int.ok()) {
    StrToCBytes(use_int.status().message(), out_error);
    return use_int.status().raw_code();
  }

  absl::StatusOr<std::unique_ptr<DistributedPointFunction>> dpf =
      CreateIncrementalDpf(params, params_size);
  if (!dpf.ok()) {
    StrToCBytes(dpf.status().message(), out_error);
    return dpf.status().raw_code();
  }

  if (!dpf.ok()) {
    StrToCBytes(dpf.status().message(), out_error);
    return dpf.status().raw_code();
  }

  absl::StatusOr<std::pair<DpfKey, DpfKey>> keys;
  std::vector<Value> values(betas_size);
  if (use_int.value()) {
    CHECK((*dpf)->RegisterValueType<ReachTuple<uint64_t>>().ok());
    for (int i = 0; i < betas_size; i++) {
      absl::StatusOr<Value> value = ToValue(ReachTuple<uint64_t>{
          betas[i].c, betas[i].rf, betas[i].r, betas[i].qf, betas[i].q});
      if (!value.ok()) {
        StrToCBytes(value.status().message(), out_error);
        return value.status().raw_code();
      }
      values[i] = *value;
    }
  } else {
    CHECK((*dpf)->RegisterValueType<ReachTuple<ReachIntModN>>().ok());
    for (int i = 0; i < betas_size; i++) {
      absl::StatusOr<Value> value = ToValue(ReachTuple<ReachIntModN>{
          ReachIntModN(betas[i].c), ReachIntModN(betas[i].rf),
          ReachIntModN(betas[i].r), ReachIntModN(betas[i].qf),
          ReachIntModN(betas[i].q)});
      if (!value.ok()) {
        StrToCBytes(value.status().message(), out_error);
        return value.status().raw_code();
      }
      values[i] = *value;
    }
  }

  keys = (*dpf)->GenerateKeysIncremental(ConvertCUInt128(alpha), values);
  if (!keys.ok()) {
    StrToCBytes(keys.status().message(), out_error);
    return keys.status().raw_code();
  }

  return CopyDpfKeys(keys.value(), out_key1, out_key2, out_error);
}

int CCreateEvaluationContext(const struct CBytes* params, int64_t params_size,
                             const struct CBytes* key,
                             struct CBytes* out_eval_context,
                             struct CBytes* out_error) {
  absl::StatusOr<std::unique_ptr<DistributedPointFunction>> dpf =
      CreateIncrementalDpf(params, params_size);
  if (!dpf.ok()) {
    StrToCBytes(dpf.status().message(), out_error);
    return dpf.status().raw_code();
  }

  DpfKey dpf_key;
  if (!dpf_key.ParseFromArray(key->c, key->l)) {
    StrToCBytes("fail to parse DpfKey", out_error);
    return static_cast<int>(absl::StatusCode::kInvalidArgument);
  }

  absl::StatusOr<EvaluationContext> eval_context =
      (*dpf)->CreateEvaluationContext(dpf_key);
  if (!eval_context.ok()) {
    StrToCBytes(eval_context.status().message(), out_error);
    return eval_context.status().raw_code();
  }

  if (!AllocateCBytes(eval_context->ByteSizeLong(), out_eval_context) ||
      !eval_context->SerializeToArray(out_eval_context->c,
                                      out_eval_context->l)) {
    StrToCBytes("fail to copy EvaluationContext", out_error);
    return static_cast<int>(absl::StatusCode::kInternal);
  }

  return static_cast<int>(absl::StatusCode::kOk);
}

int Evaluate64(bool is_multilevel, int key_bit_size, int hierarchy_level,
               const struct CUInt128* prefixes, int64_t prefixes_size,
               struct CBytes* mutable_context, struct CUInt64Vec* out_vec,
               struct CBytes* out_error) {
  EvaluationContext eval_context;
  if (!eval_context.ParseFromArray(mutable_context->c, mutable_context->l)) {
    StrToCBytes("fail to parse EvaluationContext", out_error);
    return static_cast<int>(absl::StatusCode::kInvalidArgument);
  }

  std::vector<DpfParameters> parameters;
  if (key_bit_size > 0) {
    parameters = GetDefaultDpfParameters(key_bit_size);
    *eval_context.mutable_parameters() = {parameters.begin(), parameters.end()};
  } else {
    parameters = std::vector<DpfParameters>(eval_context.parameters().begin(),
                                            eval_context.parameters().end());
  }
  absl::StatusOr<std::unique_ptr<DistributedPointFunction>> dpf =
      DistributedPointFunction::CreateIncremental(parameters);
  if (!dpf.ok()) {
    StrToCBytes(dpf.status().message(), out_error);
    return dpf.status().raw_code();
  }

  std::vector<absl::uint128> prefixes_128(prefixes_size);
  for (int i = 0; i < prefixes_size; i++) {
    prefixes_128[i] = ConvertCUInt128(&prefixes[i]);
  }

  absl::StatusOr<std::vector<uint64_t>> result;
  if (is_multilevel) {
    result = (*dpf)->EvaluateUntil<uint64_t>(hierarchy_level, prefixes_128,
                                             eval_context);
  } else {
    result = (*dpf)->EvaluateNext<uint64_t>(prefixes_128, eval_context);
  }

  if (!result.ok()) {
    StrToCBytes(result.status().message(), out_error);
    return result.status().raw_code();
  }
  free(mutable_context->c);
  if (!AllocateCBytes(eval_context.ByteSizeLong(), mutable_context) ||
      !eval_context.SerializeToArray(mutable_context->c, mutable_context->l)) {
    StrToCBytes("fail to copy EvaluationContext", out_error);
    return static_cast<int>(absl::StatusCode::kInternal);
  }

  int size = result->size();
  out_vec->vec_size = size;
  out_vec->vec = (uint64_t*)calloc(size, sizeof(uint64_t));
  if (out_vec->vec == nullptr) {
    StrToCBytes("fail to allocate memory for expanded vector", out_error);
    return static_cast<int>(absl::StatusCode::kInternal);
  }
  for (int i = 0; i < size; i++) {
    out_vec->vec[i] = (uint64_t)((*result)[i]);
  }
  return static_cast<int>(absl::StatusCode::kOk);
}

int CEvaluateNext64(const struct CUInt128* prefixes, int64_t prefixes_size,
                    struct CBytes* mutable_context, struct CUInt64Vec* out_vec,
                    struct CBytes* out_error) {
  return Evaluate64(false, 0, 0, prefixes, prefixes_size, mutable_context,
                    out_vec, out_error);
}

int CEvaluateUntil64(int hierarchy_level, const struct CUInt128* prefixes,
                     int64_t prefixes_size, struct CBytes* mutable_context,
                     struct CUInt64Vec* out_vec, struct CBytes* out_error) {
  return Evaluate64(true, 0, hierarchy_level, prefixes, prefixes_size,
                    mutable_context, out_vec, out_error);
}

int CEvaluateUntil64Default(int key_bit_size, int hierarchy_level,
                            const CUInt128* prefixes,
                            int64_t prefixes_size,
                            CBytes* mutable_context,
                            CUInt64Vec* out_vec,
                            CBytes* out_error) {
  return Evaluate64(true, key_bit_size, hierarchy_level, prefixes,
                    prefixes_size, mutable_context, out_vec, out_error);
}

int EvaluateAt64(int key_bit_size, const struct CBytes* params,
                 int64_t params_size, const struct CBytes* key,
                 int hierarchy_level, const struct CUInt128* evaluation_points,
                 int64_t evaluation_points_size, struct CUInt64Vec* out_vec,
                 struct CBytes* out_error) {
  absl::StatusOr<std::unique_ptr<DistributedPointFunction>> dpf;
  if (key_bit_size > 0) {
    dpf = DistributedPointFunction::CreateIncremental(
        std::move(GetDefaultDpfParameters(key_bit_size)));
  } else {
    dpf = CreateIncrementalDpf(params, params_size);
  }
  if (!dpf.ok()) {
    StrToCBytes(dpf.status().message(), out_error);
    return dpf.status().raw_code();
  }

  DpfKey dpf_key;
  if (!dpf_key.ParseFromArray(key->c, key->l)) {
    StrToCBytes("fail to parse DpfKey", out_error);
    return static_cast<int>(absl::StatusCode::kInvalidArgument);
  }

  std::vector<absl::uint128> evaluation_points_128(evaluation_points_size);
  for (int i = 0; i < evaluation_points_size; i++) {
    evaluation_points_128[i] = ConvertCUInt128(&evaluation_points[i]);
  }

  absl::StatusOr<std::vector<uint64_t>> result = (*dpf)->EvaluateAt<uint64_t>(
      dpf_key, hierarchy_level, evaluation_points_128);
  if (!result.ok()) {
    StrToCBytes(result.status().message(), out_error);
    return result.status().raw_code();
  }

  int size = result->size();
  out_vec->vec_size = size;
  out_vec->vec = (uint64_t*)calloc(size, sizeof(uint64_t));
  if (out_vec->vec == nullptr) {
    StrToCBytes("fail to allocate memory for expanded vector", out_error);
    return static_cast<int>(absl::StatusCode::kInternal);
  }
  std::copy(result->begin(), result->end(), out_vec->vec);
  return static_cast<int>(absl::StatusCode::kOk);
}

int CEvaluateAt64(const struct CBytes* params, int64_t params_size,
                  const struct CBytes* key, int hierarchy_level,
                  const struct CUInt128* evaluation_points,
                  int64_t evaluation_points_size, struct CUInt64Vec* out_vec,
                  struct CBytes* out_error) {
  return EvaluateAt64(0, params, params_size, key, hierarchy_level,
                      evaluation_points, evaluation_points_size, out_vec,
                      out_error);
}

int CEvaluateAt64Default(int key_bit_size, const struct CBytes* key,
                         int hierarchy_level,
                         const struct CUInt128* evaluation_points,
                         int64_t evaluation_points_size,
                         struct CUInt64Vec* out_vec, struct CBytes* out_error) {
  return EvaluateAt64(key_bit_size, nullptr, 0, key, hierarchy_level,
                      evaluation_points, evaluation_points_size, out_vec,
                      out_error);
}

int EvaluateTupleReachIntNodN(std::unique_ptr<DistributedPointFunction> dpf,
                              EvaluationContext& eval_context,
                              struct CReachTupleVec* out_vec,
                              struct CBytes* out_error) {
  absl::StatusOr<std::vector<ReachTuple<ReachIntModN>>> result =
      dpf->EvaluateNext<ReachTuple<ReachIntModN>>({}, eval_context);

  if (!result.ok()) {
    StrToCBytes(result.status().message(), out_error);
    return result.status().raw_code();
  }

  int size = result->size();
  out_vec->vec_size = size;
  out_vec->vec =
      reinterpret_cast<CReachTuple*>(calloc(size, sizeof(CReachTuple)));
  if (out_vec->vec == nullptr) {
    StrToCBytes("fail to allocate memory for expanded vector", out_error);
    return static_cast<int>(absl::StatusCode::kInternal);
  }
  for (int i = 0; i < size; i++) {
    out_vec->vec[i].c = std::get<0>((*result)[i].value()).value();
    out_vec->vec[i].rf = std::get<1>((*result)[i].value()).value();
    out_vec->vec[i].r = std::get<2>((*result)[i].value()).value();
    out_vec->vec[i].qf = std::get<3>((*result)[i].value()).value();
    out_vec->vec[i].q = std::get<4>((*result)[i].value()).value();
  }
  return static_cast<int>(absl::StatusCode::kOk);
}

int EvaluateTupleUint64(std::unique_ptr<DistributedPointFunction> dpf,
                        EvaluationContext& eval_context,
                        struct CReachTupleVec* out_vec,
                        struct CBytes* out_error) {
  absl::StatusOr<std::vector<ReachTuple<uint64_t>>> result =
      dpf->EvaluateNext<ReachTuple<uint64_t>>({}, eval_context);

  if (!result.ok()) {
    StrToCBytes(result.status().message(), out_error);
    return result.status().raw_code();
  }

  int size = result->size();
  out_vec->vec_size = size;
  out_vec->vec =
      reinterpret_cast<CReachTuple*>(calloc(size, sizeof(CReachTuple)));
  if (out_vec->vec == nullptr) {
    StrToCBytes("fail to allocate memory for expanded vector", out_error);
    return static_cast<int>(absl::StatusCode::kInternal);
  }
  for (int i = 0; i < size; i++) {
    out_vec->vec[i].c = std::get<0>((*result)[i].value());
    out_vec->vec[i].rf = std::get<1>((*result)[i].value());
    out_vec->vec[i].r = std::get<2>((*result)[i].value());
    out_vec->vec[i].qf = std::get<3>((*result)[i].value());
    out_vec->vec[i].q = std::get<4>((*result)[i].value());
  }
  return static_cast<int>(absl::StatusCode::kOk);
}

int CEvaluateReachTuple(const struct CBytes* in_context,
                        struct CReachTupleVec* out_vec,
                        struct CBytes* out_error) {
  EvaluationContext eval_context;
  if (!eval_context.ParseFromArray(in_context->c, in_context->l)) {
    StrToCBytes("fail to parse EvaluationContext", out_error);
    return static_cast<int>(absl::StatusCode::kInvalidArgument);
  }

  std::vector<DpfParameters> parameters(eval_context.parameters().begin(),
                                        eval_context.parameters().end());

  absl::StatusOr<bool> use_int = UseReachIntOrIntModN(parameters[0]);
  if (!use_int.ok()) {
    StrToCBytes(use_int.status().message(), out_error);
    return use_int.status().raw_code();
  }

  absl::StatusOr<std::unique_ptr<DistributedPointFunction>> dpf =
      DistributedPointFunction::CreateIncremental(parameters);
  if (!dpf.ok()) {
    StrToCBytes(dpf.status().message(), out_error);
    return dpf.status().raw_code();
  }

  if (use_int.value()) {
    return EvaluateTupleUint64(std::move(dpf.value()), eval_context, out_vec,
                               out_error);
  }
  return EvaluateTupleReachIntNodN(std::move(dpf.value()), eval_context,
                                   out_vec, out_error);
}

int EvaluateTupleReachIntNodNBetweenLevels(
    std::unique_ptr<DistributedPointFunction> dpf,
    EvaluationContext& eval_context, int prefix_level, int eval_level,
    struct CReachTupleVec* out_vec, struct CBytes* out_error) {
  absl::StatusOr<std::vector<ReachTuple<ReachIntModN>>> result;
  std::vector<absl::uint128> evaluation_points;
  if (prefix_level >= 0) {
    result = dpf->EvaluateAt<ReachTuple<ReachIntModN>>(
        prefix_level, std::vector<absl::uint128>{0}, eval_context);
    if (!result.ok()) {
      StrToCBytes(result.status().message(), out_error);
      return result.status().raw_code();
    }
    evaluation_points.emplace_back(absl::MakeUint128(0, 0));
  }

  result = dpf->EvaluateUntil<ReachTuple<ReachIntModN>>(
      eval_level, evaluation_points, eval_context);
  if (!result.ok()) {
    StrToCBytes(result.status().message(), out_error);
    return result.status().raw_code();
  }

  int size = result->size();
  out_vec->vec_size = size;
  out_vec->vec =
      reinterpret_cast<CReachTuple*>(calloc(size, sizeof(CReachTuple)));
  if (out_vec->vec == nullptr) {
    StrToCBytes("fail to allocate memory for expanded vector", out_error);
    return static_cast<int>(absl::StatusCode::kInternal);
  }
  for (int i = 0; i < size; i++) {
    out_vec->vec[i].c = std::get<0>((*result)[i].value()).value();
    out_vec->vec[i].rf = std::get<1>((*result)[i].value()).value();
    out_vec->vec[i].r = std::get<2>((*result)[i].value()).value();
    out_vec->vec[i].qf = std::get<3>((*result)[i].value()).value();
    out_vec->vec[i].q = std::get<4>((*result)[i].value()).value();
  }
  return static_cast<int>(absl::StatusCode::kOk);
}

int EvaluateTupleUint64BetweenLevels(
    std::unique_ptr<DistributedPointFunction> dpf,
    EvaluationContext& eval_context, int prefix_level, int eval_level,
    struct CReachTupleVec* out_vec, struct CBytes* out_error) {
  absl::StatusOr<std::vector<ReachTuple<uint64_t>>> result;
  std::vector<absl::uint128> evaluation_points;
  if (prefix_level >= 0) {
    result = dpf->EvaluateAt<ReachTuple<uint64_t>>(
        prefix_level, std::vector<absl::uint128>{0}, eval_context);
    if (!result.ok()) {
      StrToCBytes(result.status().message(), out_error);
      return result.status().raw_code();
    }
    evaluation_points.emplace_back(absl::MakeUint128(0, 0));
  }

  result = dpf->EvaluateUntil<ReachTuple<uint64_t>>(
      eval_level, evaluation_points, eval_context);
  if (!result.ok()) {
    StrToCBytes(result.status().message(), out_error);
    return result.status().raw_code();
  }

  int size = result->size();
  out_vec->vec_size = size;
  out_vec->vec =
      reinterpret_cast<CReachTuple*>(calloc(size, sizeof(CReachTuple)));
  if (out_vec->vec == nullptr) {
    StrToCBytes("fail to allocate memory for expanded vector", out_error);
    return static_cast<int>(absl::StatusCode::kInternal);
  }
  for (int i = 0; i < size; i++) {
    out_vec->vec[i].c = std::get<0>((*result)[i].value());
    out_vec->vec[i].rf = std::get<1>((*result)[i].value());
    out_vec->vec[i].r = std::get<2>((*result)[i].value());
    out_vec->vec[i].qf = std::get<3>((*result)[i].value());
    out_vec->vec[i].q = std::get<4>((*result)[i].value());
  }
  return static_cast<int>(absl::StatusCode::kOk);
}

int CEvaluateReachTupleBetweenLevels(const struct CBytes* in_context,
                                     int prefix_level, int eval_level,
                                     struct CReachTupleVec* out_vec,
                                     struct CBytes* out_error) {
  EvaluationContext eval_context;
  if (!eval_context.ParseFromArray(in_context->c, in_context->l)) {
    StrToCBytes("fail to parse EvaluationContext", out_error);
    return static_cast<int>(absl::StatusCode::kInvalidArgument);
  }

  std::vector<DpfParameters> parameters(eval_context.parameters().begin(),
                                        eval_context.parameters().end());

  absl::StatusOr<bool> use_int = UseReachIntOrIntModN(parameters[0]);
  if (!use_int.ok()) {
    StrToCBytes(use_int.status().message(), out_error);
    return use_int.status().raw_code();
  }

  absl::StatusOr<std::unique_ptr<DistributedPointFunction>> dpf =
      DistributedPointFunction::CreateIncremental(parameters);
  if (!dpf.ok()) {
    StrToCBytes(dpf.status().message(), out_error);
    return dpf.status().raw_code();
  }

  if (use_int.value()) {
    return EvaluateTupleUint64BetweenLevels(std::move(dpf.value()),
                                            eval_context, prefix_level,
                                            eval_level, out_vec, out_error);
  }
  return EvaluateTupleReachIntNodNBetweenLevels(std::move(dpf.value()),
                                                eval_context, prefix_level,
                                                eval_level, out_vec, out_error);
}

uint64_t CCreateReachIntModN(uint64_t v) { return ReachIntModN(v).value(); }

void CCreateReachIntModNTuple(struct CReachTuple* tuple) {
  tuple->c = CCreateReachIntModN(tuple->c);
  tuple->rf = CCreateReachIntModN(tuple->rf);
  tuple->r = CCreateReachIntModN(tuple->r);
  tuple->qf = CCreateReachIntModN(tuple->qf);
  tuple->q = CCreateReachIntModN(tuple->q);
}

uint64_t CAddReachIntModN(uint64_t a, uint64_t b) {
  return (ReachIntModN(a) + ReachIntModN(b)).value();
}

void CAddReachIntModNTuple(struct CReachTuple* tuple_a,
                           const struct CReachTuple* tuple_b) {
  tuple_a->c = CAddReachIntModN(tuple_a->c, tuple_b->c);
  tuple_a->rf = CAddReachIntModN(tuple_a->rf, tuple_b->rf);
  tuple_a->r = CAddReachIntModN(tuple_a->r, tuple_b->r);
  tuple_a->qf = CAddReachIntModN(tuple_a->qf, tuple_b->qf);
  tuple_a->q = CAddReachIntModN(tuple_a->q, tuple_b->q);
}

uint64_t GetReachModule() { return reach_module; }
