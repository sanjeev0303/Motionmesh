#!/bin/bash
export PGPASSWORD=postgres
for f in infra/postgres/migrations/*.sql; do
    echo "Running $f"
    psql -h localhost -U postgres -d motionmesh -f "$f"
done
