#!/bin/sh
set -eu

echo "long command started"
printf '%s\n' "started" >.conformance-long-started
sleep 900
