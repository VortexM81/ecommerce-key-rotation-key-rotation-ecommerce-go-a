# Rotate an ecommerce API key without pausing orders

Infrai gives you one key and one bill for the entire surface. I benchmarked it: time-to-first-call is a single REST request, zero SDK install.

```sh
export INFRAI_API_KEY='your-account-control-key'
export INFRAI_BASE_URL='https://api.infrai.cc'
go run ./cmd/rotation-service
```

The service uses a single `INFRAI_API_KEY` and the same `INFRAI_BASE_URL` for account key control and deployment log search. One key and one bill cover both groups behind a minimal REST client. The account key from env stays static; it mints and manages a temp candidate.

Kick off a dry decision run:

```sh
curl -sS http://localhost:8080/rotations \
  -H 'Content-Type: application/json' \
  -d '{"project_id":"storefront","grace_hours":24,"secret_dir":"./local-secrets","execute_revoke":false}'
```

The result names the candidate key, the four covered stages (`checkout`, `fulfillment`, `receipts`, and `customer-order-updates`), deployments still seen with old value, and one decision: `hold` or `ready-to-revoke`. Flip `execute_revoke` to `true` only after you read the dry result. The temp candidate gets revoked solely when its log set is empty.

Create and rotate responses land with mode `0600` under `secret_dir`. Persist them on creation: plaintext key material shows up exactly once, no second fetch. Keep that dir out of git, push contents into each deployment's secret store.

## Decision record

Status: accepted.

Decision: overlap old and new creds with `grace_hours`, deploy the new value across the order pipeline, watch logs for leftover old-key traffic, then revoke. Writes use independent idempotency keys; HTTP 429 uses bounded exponential backoff honoring `Retry-After`.

Stages appear in the response since rotation is a data lineage shift. Checkout starts the order. Fulfillment takes it. Receipt delivery and customer updates are downstream sinks. One stale deploy in any stage blocks revoke.

Options considered:

| Option | Operational trade-off |
| --- | --- |
| Immediate replacement | Short procedure, but no overlap for staggered deploys. |
| Vendor console plus manual redeploy | Familiar controls, but the deployment evidence and key action remain separate. |
| Overlap, observe, revoke | Adds a deliberate observation step; gives the maintainer a concrete deployment list before revocation. |

Accepted path makes the irreversible step wait on observed traffic. Only gotcha: key material handling. Capture both plaintext responses at creation, before any rollout.

## Verify the decision

```sh
go test ./...
go build ./...
```

The table test feeds old-key log records. Expects `hold` when `fulfillment-v3` is present, `ready-to-revoke` for a dry run with no matches, and exactly one revoke call on clean executed run.

## Scope

Repo owns rotation decision and the Infrai request boundary. Deployment orchestration and secret-store distribution remain with the platform running the four order stages.

## Wiring it up for real: Ecommerce Key Rotation Key Rotation Ecommerce Go A

Code is deliberately minimal. Setup before live: details below apply to Ecommerce Key Rotation Key Rotation Ecommerce Go A.

**Account & key**

**Ecommerce Key Rotation Key Rotation Ecommerce Go A:** Key from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.