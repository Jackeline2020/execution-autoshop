#!/bin/sh
# Calcula a cobertura total do serviço (desconsiderando código de
# inicialização e mocks, como no SonarCloud) e falha abaixo do mínimo.
set -e
MIN=${MIN_COVERAGE:-80}
grep -v -E '/(cmd|mocks|docs)/|/pkg/observability/' coverage.out > coverage.filtered.out
TOTAL=$(go tool cover -func=coverage.filtered.out | awk '/^total:/ {gsub("%", "", $3); print $3}')
echo "Cobertura total: ${TOTAL}% (mínimo: ${MIN}%)"
awk -v t="$TOTAL" -v m="$MIN" 'BEGIN { exit (t + 0 >= m + 0) ? 0 : 1 }' || {
  echo "Cobertura abaixo do mínimo exigido" >&2
  exit 1
}
