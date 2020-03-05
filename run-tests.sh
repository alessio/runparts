#!/bin/bash

fail() {
  echo "FAIL - $1" >&2
  exit 1
}

run_test_case() {
  local scripts want wantExit outputs name testcasesdir rootdir runparts

  runparts=$1
  rootdir=$2
  name=$(basename ${rootdir})
  scripts=${rootdir}/scripts
  args=${rootdir}/args
  gotStderr=${rootdir}/gotStderr
  gotStdout=${rootdir}/gotStdout
  wantStderr=${rootdir}/wantStderr
  wantStdout=${rootdir}/wantStdout
  wantExit=${rootdir}/wantExit

  rm -f ${gotStdout} ${gotStderr}
  echo -n -e "[${name}] ${runparts} $(<${args}) ${scripts} 1>${gotStdout} 2>${gotStderr} " >&2
  ${runparts} $(<${args}) ${scripts} 1>${gotStdout} 2>${gotStderr}
  [ $? = $(<${wantExit}) ] || fail exitCode
  diff -u ${wantStdout} ${gotStdout} || fail stdout
  diff -u ${wantStderr} ${gotStderr} || fail stderr
  echo "OK" >&2
}

set -eo pipefail;
[[ $TEST_TRACE ]] && set -x
set -u

runparts=$1
testdata=$2

[ -x ${runparts} ]
[ -d ${testdata} ]

testcases="$(ls -d ${testdata}/TestCase*)"
for tc in ${testcases}; do
  run_test_case ${runparts} ${tc}
done
