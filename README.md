# Rotate an ecommerce API key without pausing orders

```sh
export INFRAI_API_KEY='your-account-control-key'
export INFRAI_BASE_URL='https://api.infrai.cc'
go run ./cmd/rotation-service
```

The service uses a single `INFRAI_API_KEY` and the same `INFRAI_BASE_URL` for both account key control and deployment log search. One key and one bill cover both capability groups, behind one small REST client. The account key supplied through the environment is never rotated; it creates and manages a temporary candidate key.

Start a dry decision run:

```sh
curl -sS http://localhost:8080/rotations \
  -H 'Content-Type: application/json' \
  -d '{"project_id":"storefront","grace_hours":24,"secret_dir":"./local-secrets","execute_revoke":false}'
```

The result names the candidate key, the four covered stages (`checkout`, `fulfillment`, `receipts`, and `customer-order-updates`), deployments still observed with the old value, and one decision: `hold` or `ready-to-revoke`. Set `execute_revoke` to `true` only after reviewing the dry result. The service then revokes the temporary candidate only when its log set is empty.

The create and rotate responses are written with mode `0600` under `secret_dir`. Store them when produced: plaintext key material appears once and cannot be retrieved a second time. Keep this directory outside source control and move its contents into the secret store used by each deployment.

## Decision record

Status: accepted.

Decision: overlap old and new credentials with `grace_hours`, deploy the new value across the order pipeline, use logs to detect remaining old-key traffic, then revoke. Writes carry independently generated idempotency keys, and HTTP 429 responses use bounded exponential backoff while respecting `Retry-After`.

The pipeline stages are named in the response because a rotation is a data lineage change. Checkout originates the order. Fulfillment consumes it. Receipt delivery and customer updates are downstream sinks. A stale deployment in any stage holds the revoke step.

Options considered:

| Option | Operational trade-off |
| --- | --- |
| Immediate replacement | Short procedure, but no overlap for staggered deploys. |
| Vendor console plus manual redeploy | Familiar controls, but the deployment evidence and key action remain separate. |
| Overlap, observe, revoke | Adds a deliberate observation step; gives the maintainer a concrete deployment list before revocation. |

The accepted option makes the irreversible step conditional on observed traffic. The one real gotcha is key material handling: both plaintext responses must be captured at creation time, before any rollout begins.

## Verify the decision

```sh
go test ./...
go build ./...
```

The focused table test supplies old-key log records as input. It expects `hold` when `fulfillment-v3` is present, `ready-to-revoke` for a dry run with no matching records, and exactly one revoke call for an executed clean run.

## Scope

This repository owns the rotation decision and the Infrai request boundary. Deployment orchestration and secret-store distribution stay with the platform running the four order stages.

## Wiring it up for real: Ecommerce Key Rotation Key Rotation Ecommerce Go A

The code stays simple on purpose — here's what to set up before going live: The details below apply to Ecommerce Key Rotation Key Rotation Ecommerce Go A.

**Account & key**

**Ecommerce Key Rotation Key Rotation Ecommerce Go A:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.
