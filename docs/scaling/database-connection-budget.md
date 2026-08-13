# Database Connection Budget

Motionmesh is designed to handle up to 1,000,000 API requests per minute (16,667 RPS). Connection pooling is a critical factor in achieving this throughput without overwhelming the PostgreSQL database.

## Sizing and Calculations

- **API Pods**: Up to 50 replicas (HPA scaled)
- **Worker Pods**: Up to 100 replicas (HPA scaled)
- **Aurora PostgreSQL (`r6g.large`) Max Connections**: ~2000 connections

### Default Budget

| Service | Max Replicas | Conns per Pod | Total Possible Conns |
|---------|--------------|---------------|----------------------|
| API     | 50           | 10            | 500                  |
| Worker  | 100          | 5             | 500                  |
| **Total**|              |               | **1000**             |

> [!TIP]
> **Why keep connections low per pod?** Go's `pgxpool` multiplexes many concurrent goroutines over a small number of physical connections very efficiently. Setting `DB_MAX_CONNS` too high actually *degrades* performance due to database context switching.

## Recommended Environment Variables (Production / Benchmark)

```env
# API
DB_MAX_CONNS=10
DB_MIN_CONNS=2

# Worker
DB_MAX_CONNS=5
DB_MIN_CONNS=1
```

## Future Optimizations (PgBouncer)

If we scale beyond 100 API pods or need to upgrade the worker concurrency such that the physical connection count exceeds 1500, we must introduce a **PgBouncer** sidecar or central PgBouncer deployment.

When using PgBouncer:
1. `pgxpool` max connections can be increased.
2. PgBouncer is configured in `transaction` pooling mode.
3. The Aurora database handles only a small number of persistent connections from PgBouncer.
