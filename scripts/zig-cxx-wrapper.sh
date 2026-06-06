#!/bin/bash
ARGS=()
for arg in "$@"; do
  if [ "$arg" != "-mthreads" ]; then
    ARGS+=("$arg")
  fi
done
exec zig c++ -target x86_64-windows-gnu "${ARGS[@]}"
