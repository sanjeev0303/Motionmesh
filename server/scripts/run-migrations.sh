#!/bin/bash
if [ -f server/.env ]; then
    export $(grep -v '^#' server/.env | xargs)
fi

DB_CONN="${DATABASE_URL:-postgresql://postgres:postgres@localhost:5432/motionmesh}"

if command -v psql &> /dev/null; then
    for f in infra/postgres/migrations/*.sql; do
        echo "Running $f"
        psql "$DB_CONN" -f "$f"
    done
else
    echo "psql not found, running via docker"
    docker run --rm -v $(pwd)/infra/postgres/migrations:/migrations \
        -e DATABASE_URL="$DB_CONN" \
        postgres:15-alpine sh -c 'for f in /migrations/*.sql; do echo "Running $f"; psql "$DATABASE_URL" -f "$f"; done'
fi
