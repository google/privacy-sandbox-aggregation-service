/**
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

# Example values required by operator_service.tf
#
# These values should be modified for each of your environments.

region      = "eu-central-1"
environment = "operator-demo-env"

# Total resources available affected by instance_type -- actual resources used
# is affected by enclave_cpu_count / enclave_memory_mib. All 3 values should be
# updated at the same time.
instance_type      = "m5.2xlarge" # 8 cores, 32GiB
enclave_cpu_count  = 6            # Leave 2 vCPUs to host OS (minimum required).
enclave_memory_mib = 28672        # 28 GiB. ~30GiB are available on m5.2xlarge, leave 2GiB for host OS.

max_job_num_attempts_parameter      = "5"
max_job_processing_time_parameter   = "3600"
coordinator_a_assume_role_parameter = "arn:aws:iam::504220136257:role/a_211963930287_coordinator_assume_role"
# Remove coordinator_b_assume_role_parameter if using single coordinator
coordinator_b_assume_role_parameter = "arn:aws:iam::204199059659:role/b_211963930287_coordinator_assume_role"

min_capacity_ec2_instances = "1"
max_capacity_ec2_instances = "20"

alarm_notification_email = "harsh.chauhan@adjust.com"
