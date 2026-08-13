# Database Connection Budget

This document outlines the connection pooling topology for Motionmesh to safely sustain 20,000+ RPS without overwhelming the underlying PostgreSQL (Aurora) instance.

## Topology

Motionmesh relies on a 3-tier database connection topology:

1. **Application-Level Pools (`pgxpool`)**: Each Go application maintains a bounded pool of connections.
2. **Physical Database (PostgreSQL/Aurora)**: Sustains a maximum physical connection limit depending on instance size.

## Connection Budget Calculation

With a target physical database limit of **~3000 maximum connections** on Aurora depending on ACU/instance class:

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
Our `2,860` potential logical connections must fit within Aurora's max_connections limit. Because K8s pods use `pgxpool`, they maintain their own small pools and connection churn is minimized. To sustain this, the Aurora instance must be scaled to support at least `3000` concurrent connections, or PgBouncer must be introduced to K8s in the future.
