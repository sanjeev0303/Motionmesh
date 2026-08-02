# Motionmesh

S3-compatible storage, video transcoding, and CDN delivery — infrastructure a developer builds on, not a consumer product.

[![npm package](https://img.shields.io/npm/v/@motionmesh/sdk)](https://www.npmjs.com/package/@motionmesh/sdk)
[![PyPI version](https://img.shields.io/pypi/v/motionmesh)](https://pypi.org/project/motionmesh/)
[![Build Status](https://img.shields.io/github/actions/workflow/status/sanjeev0303/motionmesh/ci.yml?branch=main)](https://github.com/sanjeev0303/motionmesh/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Quickstart

```javascript
import { MotionmeshClient } from "@motionmesh/sdk";

const client = new MotionmeshClient({ apiKey: process.env.MOTIONMESH_API_KEY });

const video = await client.mediaConverter.createJob({
  file: myVideoFile,
  bucketId: process.env.MOTIONMESH_BUCKET_ID,
});

const playbackUrl = await client.videos.getPlaybackUrl(video.id);
```

```python
from motionmesh import Client

client = Client(api_key="mot_live_...")
video = client.videos.upload("input.mp4", bucket_id="...")
playback_url = client.videos.get_playback_url(video.id)
```

```bash
npm install @motionmesh/sdk @motionmesh/player
pip install motionmesh
```

## What's included

- **Storage** — S3-compatible object storage, dual-bucket support (separate source-upload and transcode-output buckets, or one bucket for both).
- **Media Convert** — self-hosted FFmpeg pipeline: 5-rendition ABR ladder (1080p down to 240p, HLS/CMAF), auto-generated captions and transcripts, topic-based chapter markers, optional watermarking, thumbnails/preview clips/scrub-sprite generation.
- **CDN** — Cloudflare-backed edge delivery, signed playback URLs, bring-your-own custom domain.

## The player

```jsx
import { MotionmeshPlayer } from "@motionmesh/player";

<MotionmeshPlayer src={playbackUrl} />
```

Built on Vidstack. Resolution switching is automatic (ABR) by default.

## Authentication

Motionmesh enforces a strict server-to-server security model via a proxy pattern. The browser-side SDK **never** holds a real API key. Exposing your `MOTIONMESH_API_KEY` to the client is a severe security vulnerability.

Instead, a small backend route using the SDK's exported request handler securely holds the API key server-side. The client SDK communicates with this trusted proxy route, which then forwards the authenticated requests to the Motionmesh API. See the [Authentication Guide](https://motionmesh.com/docs/auth) for complete Next.js and Express examples.

## Project structure

```
motionmesh/
├── server/        # Go API, worker fleet, Python captions sidecar, Cloudflare CDN worker
├── client/        # Next.js dashboard
├── sdk/           # @motionmesh/sdk, @motionmesh/player, Python SDK
├── docs/          # Fumadocs site
└── infra/         # Terraform, k8s configurations
```

## Local development

```bash
git clone https://github.com/sanjeev0303/motionmesh.git
cd motionmesh
cp .env.example .env   # fill in real values — see Environment Variables below
docker compose up
```

For production deployment instructions, please see the [Deployment Guide](https://motionmesh.com/docs/deployment).

## Environment variables

| Variable | Required for | Notes |
|---|---|---|
| `DATABASE_URL` | Everything | Neon Postgres connection string |
| `REDIS_URL` | API, caching | |
| `QUEUE_URL` | API, worker | NATS |
| `STORAGE_ENDPOINT` | Storage, worker | Backblaze B2 credentials |
| `STORAGE_ACCESS_KEY` | Storage, worker | |
| `STORAGE_SECRET_KEY` | Storage, worker | |
| `STORAGE_BUCKET` | Storage, worker | |
| `STORAGE_REGION` | Storage, worker | |
| `STORAGE_USE_SSL` | Storage, worker | |
| `JWT_SECRET` | API key signing | Generate fresh per environment — never reuse across dev/staging/prod |
| `CLERK_SECRET_KEY` | Dashboard auth | |
| `CLERK_JWKS_URL` | Dashboard auth | JWKS endpoint for networkless JWT verification |
| `STRIPE_SECRET_KEY` | Billing | |
| `STRIPE_WEBHOOK_SECRET` | Billing | |
| `CLOUDFLARE_API_TOKEN` | CDN | |
| `CLOUDFLARE_ZONE_ID` | CDN | |
| `CLOUDFLARE_ACCOUNT_ID`| CDN | |
| `CDN_SIGNING_SECRET` | CDN | |
| `CDN_FALLBACK_ORIGIN` | CDN | |

## Documentation

Full documentation and API reference are available at [motionmesh.com/docs](https://motionmesh.com/docs).

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on how to run tests, submit pull requests, and code style expectations.

## Community / Support

If you have questions or need assistance, please open a discussion in [GitHub Discussions](https://github.com/sanjeev0303/motionmesh/discussions) or contact us at `support@motionmesh.com`.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
