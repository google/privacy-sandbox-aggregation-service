#!/bin/bash

# This script builds and runs the tests in browsersimulator_test.go,
# creates a memory or cpu profile, and runs pprof on it to view how much memory/cpu
# is being used, and where. This was used to bring the memory usage of the
# pipeline down by orders of magnitude.

source gbash.sh || exit

DEFINE_enum profile --enum="memory,cpu" "cpu" "Whether to profile memory usage or cpu performance"

GOOGLE3=$(pwd | sed "s/\(.*google3\).*/\1/")
if [ ! -d $GOOGLE3 ]; then
  echo This script must be run from inside some google3 tree.
  exit 1
fi

gbash::init_google "$@"
set -ex
PROFILE_FILE=mem.prof
if [[ "$FLAGS_profile" == "cpu" ]]; then
  PROFILE_FILE=cpu.prof
fi

blaze build -c opt //third_party/privacy_sandbox_aggregation/test:dpfdataconverter_test
TEST=$GOOGLE3/blaze-bin/third_party/privacy_sandbox_aggregation/test/dpfdataconverter_test
# Change the test.bench argument to the name of a different function to
# benchmark.
GOGC=off $TEST \
  -test.bench=BenchmarkPipeline \
  -test.benchtime=5x \
  -test.v \
  -test.memprofile=`pwd`/mem.prof \
  -test.cpuprofile=`pwd`/cpu.prof &&

# Creates an SVG of the memory usage and opens it in a web browser
go tool pprof --web --nodefraction=0.1 $TEST $PROFILE_FILE
