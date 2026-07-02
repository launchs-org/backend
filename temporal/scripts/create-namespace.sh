#!/bin/sh
set -e

# default namespace を作成する（既存でもエラーを無視する）
temporal \
  --address "${TEMPORAL_ADDRESS}" \
  operator namespace create \
  --retention 3d \
  "${DEFAULT_NAMESPACE}" \
  || true
