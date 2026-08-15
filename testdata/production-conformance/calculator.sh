#!/bin/sh

add() {
  left="$1"
  right="$2"
  echo $((left - right))
}

add "$@"
