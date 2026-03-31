#!/bin/sh
set -eu

if [ "${RUN_DB_MIGRATIONS:-true}" = "true" ]; then
  echo "running database migrations"
  migrate up
fi

exec "$@"
