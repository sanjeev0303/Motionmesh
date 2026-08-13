# Database Connection Budget

This document outlines the connection pooling topology for Motionmesh to safely sustain 20,000+ RPS without overwhelming the underlying PostgreSQL (Aurora) instance.

## Topology

Motionmesh relies on a 3-tier database connection topology:

1. **Application-Level Pools (`pgxpool`)**: Each Go application maintains a bounded pool of connections.
2. **PgBouncer (Transaction Mode)**: Acts as a central multiplexer, mapping thousands of logical application connections onto a smaller physical pool.
3. **Physical Database (PostgreSQL/Aurora)**: Sustains a maximum physical connection limit (e.g., 400).

## Connection Budget Calculation

With a target physical database limit of **400 maximum connections**, PgBouncer is configured with:
- `max_client_conn = 10000`
- `default_pool_size = 350`

### API Tier
- **Replicas**: 50 (Maximum HPA)
- **`DB_MAX_CONNS` (per pod)**: 15
- **Maximum Logical Connections**: 50 * 15 = 750

### Worker Tier
- **Replicas**: 100 (Maximum HPA)
- **`DB_MAX_CONNS` (per pod)**: 20
- **Maximum Logical Connections**: 100 * 20 = 2000

### Sidecars & Auxiliaries
- **Billing Worker**: 10 pods * 10 conns = 100 logical
- **Cleanup Worker**: 2 pods * 5 conns = 10 logical

**Total Potential Logical Connections**: ~2,860

## Conclusion
Our `2,860` potential logical connections comfortably fit within PgBouncer's `max_client_conn` of `10,000`. PgBouncer will rapidly multiplex these transactions over its `350` physical connections to the Aurora database. Because `pgxpool` utilizes short-lived transactions and `pgbouncer` is in transaction mode, the database will never experience connection starvation, even at peak 20,000 RPS.
