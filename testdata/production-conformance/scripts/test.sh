#!/bin/sh
set -eu

actual="$(./calculator.sh 7 5)"
if [ "${actual}" != "12" ]; then
  echo "calculator result ${actual}, want 12" >&2
  exit 1
fi

echo "fixture tests passed"
