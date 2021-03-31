#include "pipeline/distributed_point_function_c_bridge.h"

#include <cstdint>
#include <cstdlib>
#include <vector>

#include "absl/numeric/int128.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/string_view.h"
#include "absl/types/span.h"
#include "pipeline/cbytes.h"
#include "pipeline/cbytes_utils.h"
#include "dpf/distributed_point_function.h"
#include "dpf/distributed_point_function.pb.h"

using ::convagg::crypto::StrToCBytes;
using ::private_statistics::dpf::Block;
using ::private_statistics::dpf::CorrectionWord;
using ::private_statistics::dpf::DistributedPointFunction;
using ::private_statistics::dpf::DpfKey;
using ::private_statistics::dpf::DpfParameters;
using ::private_statistics::dpf::EvaluationContext;
using ::private_statistics::dpf::PartialEvaluation;

void DpfParametersProtoToC(struct CDpfParameters* c_p, const DpfParameters& p) {
  c_p->log_domain_size = p.log_domain_size();
  c_p->element_bitsize = p.element_bitsize();
}

DpfParameters DpfParametersCToProto(const struct CDpfParameters* c_p) {
  DpfParameters param;
  param.set_log_domain_size(c_p->log_domain_size);
  param.set_element_bitsize(c_p->element_bitsize);
  return param;
}

void BlockProtoToC(struct CBlock* c_b, const Block& b) {
  c_b->high = b.high();
  c_b->low = b.low();
}

Block BlockCToProto(const struct CBlock* c_b) {
  Block b;
  b.set_high(c_b->high);
  b.set_low(c_b->low);
  return b;
}

void CorrectionWordProtoToC(CCorrectionWord* c_c, const CorrectionWord& c) {
  BlockProtoToC(&c_c->seed, c.seed());
  c_c->control_left = c.control_left();
  c_c->control_right = c.control_right();
  BlockProtoToC(&c_c->output, c.output());
}

CorrectionWord CorrectionWordCToProto(const CCorrectionWord* c_c) {
  CorrectionWord cw;
  *cw.mutable_seed() = BlockCToProto(&c_c->seed);
  cw.set_control_left(c_c->control_left);
  cw.set_control_right(c_c->control_right);
  *cw.mutable_output() = BlockCToProto(&c_c->output);
  return cw;
}

void DpfKeyProtoToC(struct CDpfKey* c_d, const DpfKey d) {
  BlockProtoToC(&c_d->seed, d.seed());

  int cw_size = d.correction_words_size();
  c_d->correction_words_size = cw_size;
  c_d->correction_words =
      (CCorrectionWord*)calloc(cw_size, sizeof(struct CorrectionWord));
  for (int i = 0; i < cw_size; i++) {
    CorrectionWordProtoToC(&c_d->correction_words[i], d.correction_words(i));
  }

  c_d->party = d.party();

  BlockProtoToC(&c_d->last_level_output_correction,
                d.last_level_output_correction());
}

DpfKey DpfKeyCToProto(const struct CDpfKey* c_d) {
  DpfKey key;
  *key.mutable_seed() = BlockCToProto(&c_d->seed);
  for (int i = 0; i < c_d->correction_words_size; i++) {
    *key.add_correction_words() =
        CorrectionWordCToProto(&c_d->correction_words[i]);
  }
  key.set_party(c_d->party);
  *key.mutable_last_level_output_correction() =
      BlockCToProto(&c_d->last_level_output_correction);
  return key;
}

void PartialEvaluationProtoToC(struct CPartialEvaluation* c_p,
                               const PartialEvaluation& p) {
  BlockProtoToC(&c_p->prefix, p.prefix());
  BlockProtoToC(&c_p->seed, p.seed());
  c_p->control_bit = p.control_bit();
}

PartialEvaluation PartialEvaluationCToProto(
    const struct CPartialEvaluation* c_p) {
  PartialEvaluation pe;
  *pe.mutable_prefix() = BlockCToProto(&c_p->prefix);
  *pe.mutable_seed() = BlockCToProto(&c_p->seed);
  pe.set_control_bit(c_p->control_bit);
  return pe;
}

void EvaluationContextProtoToC(struct CEvaluationContext* c_e,
                               const EvaluationContext& e) {
  int param_size = e.parameters_size();
  c_e->parameters_size = param_size;
  c_e->parameters =
      (CDpfParameters*)calloc(param_size, sizeof(struct CDpfParameters));
  for (int i = 0; i < param_size; i++) {
    DpfParametersProtoToC(&c_e->parameters[i], e.parameters(i));
  }

  DpfKeyProtoToC(&c_e->key, e.key());

  c_e->hierarchy_level = e.hierarchy_level();

  int pe_size = e.partial_evaluations_size();
  c_e->partial_evaluations_size = pe_size;
  c_e->partial_evaluations =
      (CPartialEvaluation*)calloc(pe_size, sizeof(struct PartialEvaluation));
  for (int i = 0; i < pe_size; i++) {
    PartialEvaluationProtoToC(&c_e->partial_evaluations[i],
                              e.partial_evaluations(i));
  }
}

EvaluationContext EvaluationContextCToProto(
    const struct CEvaluationContext* c_e) {
  EvaluationContext e;
  for (int i = 0; i < c_e->parameters_size; i++) {
    *e.add_parameters() = DpfParametersCToProto(&c_e->parameters[i]);
  }
  *e.mutable_key() = DpfKeyCToProto(&c_e->key);
  e.set_hierarchy_level(c_e->hierarchy_level);
  for (int i = 0; i < c_e->partial_evaluations_size; i++) {
    *e.add_partial_evaluations() =
        PartialEvaluationCToProto(&c_e->partial_evaluations[i]);
  }
  return e;
}

int CGenerateKeys(const struct CDpfParameters* param, uint64_t alpha,
                  uint64_t beta, struct CDpfKeyPair* out_dpf_keys,
                  struct CBytes* out_error) {
  DpfParameters parameters = DpfParametersCToProto(param);
  absl::StatusOr<std::unique_ptr<DistributedPointFunction>> dpf =
      DistributedPointFunction::Create(parameters);
  if (!dpf.ok()) {
    StrToCBytes(dpf.status().message(), out_error);
    return dpf.status().raw_code();
  }

  absl::StatusOr<std::pair<DpfKey, DpfKey>> keys =
      dpf.value()->GenerateKeys(absl::uint128(alpha), absl::uint128(beta));
  if (!keys.ok()) {
    StrToCBytes(keys.status().message(), out_error);
    return keys.status().raw_code();
  }

  DpfKeyProtoToC(&out_dpf_keys->key1, keys->first);
  DpfKeyProtoToC(&out_dpf_keys->key2, keys->second);
  return static_cast<int>(absl::StatusCode::kOk);
}

int CCreateEvaluationContext(const struct CDpfParameters* param,
                             const struct CDpfKey* key,
                             struct CEvaluationContext* out_eval_context,
                             struct CBytes* out_error) {
  DpfParameters parameters = DpfParametersCToProto(param);
  absl::StatusOr<std::unique_ptr<DistributedPointFunction>> dpf =
      DistributedPointFunction::Create(parameters);
  if (!dpf.ok()) {
    StrToCBytes(dpf.status().message(), out_error);
    return dpf.status().raw_code();
  }

  absl::StatusOr<EvaluationContext> eval_context =
      (*dpf)->CreateEvaluationContext(DpfKeyCToProto(key));
  if (!eval_context.ok()) {
    StrToCBytes(eval_context.status().message(), out_error);
    return eval_context.status().raw_code();
  }

  EvaluationContextProtoToC(out_eval_context, eval_context.value());
  return static_cast<int>(absl::StatusCode::kOk);
}

int CEvaluateNext64(const struct CDpfParameters* param,
                    const uint64_t* prefixes, uint64_t prefixes_size,
                    CEvaluationContext* mutable_context,
                    struct CUInt64Vec* out_vec, struct CBytes* out_error) {
  DpfParameters parameters = DpfParametersCToProto(param);
  absl::StatusOr<std::unique_ptr<DistributedPointFunction>> dpf =
      DistributedPointFunction::Create(parameters);
  if (!dpf.ok()) {
    StrToCBytes(dpf.status().message(), out_error);
    return dpf.status().raw_code();
  }

  std::vector<absl::uint128> prefixes_128(prefixes_size);
  for (int i = 0; i < prefixes_size; i++) {
    prefixes_128[i] = absl::uint128(prefixes[i]);
  }
  EvaluationContext eval_context = EvaluationContextCToProto(mutable_context);
  auto maybe_result =
      dpf.value()->EvaluateNext<uint64_t>(prefixes_128, eval_context);
  if (!maybe_result.ok()) {
    StrToCBytes(maybe_result.status().message(), out_error);
    return maybe_result.status().raw_code();
  }
  free(mutable_context->parameters);
  free(mutable_context->partial_evaluations);
  free(mutable_context->key.correction_words);
  EvaluationContextProtoToC(mutable_context, eval_context);

  int size = maybe_result->size();
  out_vec->vec_size = size;
  out_vec->vec = (uint64_t*)calloc(size, sizeof(uint64_t));
  for (int i = 0; i < size; i++) {
    out_vec->vec[i] = (uint64_t)((*maybe_result)[i]);
  }
  return static_cast<int>(absl::StatusCode::kOk);
}
