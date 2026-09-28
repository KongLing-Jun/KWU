# ADR-0001: Use one verified development Cloud environment contract

## Status

Accepted

## Date

2026-08-11

## Context

Desktop, Cloud, and Admin currently resolve their development Cloud independently. Desktop has a source default and an environment-variable override, Admin has a Vite proxy default, and Cloud prioritizes a worktree-local `.env`. These sources can disagree while every individual process still appears healthy.

A changed Desktop Cloud URL selects a different development profile. Reusing the same URL with another database is also unsafe because URL identity alone cannot distinguish deployments. Copying an ignored `.env` between worktrees makes configuration drift durable and risks moving secrets into additional locations.

The approved development topology is a locally compiled Cloud listener on `127.0.0.1:9090`, backed by the team PostgreSQL environment and local Redis. Developers may provision secrets once at machine scope, but must not maintain an `.env` per worktree.

## Decision

Use a single verified development environment contract:

1. Cloud configuration is resolved through one precedence chain that supports an explicit managed path, process overrides, a user-level machine configuration, and checked-in non-secret defaults. Worktree `.env` is not an implicit source.
2. A canonical launcher resolves the Cloud URL once and starts Cloud, Admin, and Desktop from that value.
3. The launcher writes a non-secret runtime manifest containing topology, source revisions, process ownership, and a stable Cloud environment identifier.
4. Cloud exposes the stable identifier from authoritative database state. Desktop persists and verifies it in addition to the normalized URL.
5. Startup, cutover, status, and shutdown fail closed when URL, identity, PID, port, worktree, or revision ownership is inconsistent.
6. Legacy `.env` and inherited endpoint overrides are surfaced as explicit migration/conflict errors rather than silently applied.

## Alternatives Considered

### Keep copying `.env` into every worktree

- Advantage: no code changes.
- Rejected: duplicates secrets, preserves stale endpoints, and guarantees drift between worktrees.

### Standardize only the Cloud URL

- Advantage: small launcher change.
- Rejected: the same URL can later point to a different database or deployment; URL equality is not sufficient environment identity.

### Commit all development configuration

- Advantage: true zero-step checkout.
- Rejected: the approved topology uses a shared PostgreSQL environment and external integrations whose credentials must not enter Git.

### Make every developer use a fully local disposable stack

- Advantage: no shared database credentials and stronger isolation.
- Rejected for this decision: it changes the approved team topology. It remains a possible future profile, not the standard integrated environment.

## Consequences

- Developers provision machine-level secrets once and no longer copy `.env` files across worktrees.
- Desktop and Admin cannot independently drift to different Cloud endpoints.
- A database/deployment mismatch becomes a startup error instead of a misleading empty or logged-out application state.
- The launcher becomes the supported integrated development entrypoint; direct component commands remain useful for focused tests but do not prove full-stack binding.
- Cloud configuration precedence changes and therefore requires compatibility tests and an explicit legacy migration path.
- The environment identity may require a small database-backed contract. If a schema migration is necessary, implementation pauses for separate approval as required by the specification boundary.

## Implementation Discovery: Database Identity

The Task 5 audit found no existing database-owned unique environment identifier. `PROJECTS_INSTANCE_NAME` is process configuration, Goose version and migration fingerprints identify schema generation rather than a database instance, and payment environment values are business data rather than deployment identity.

The durable implementation uses separately approved migration `00090`, with a singleton `cloud_environment_identity` row containing a stable UUID, a non-secret environment name, and creation metadata. Cloud reads but does not silently replace this identity during ordinary startup. A cloned database must explicitly rotate its identity before it can be used as another environment.

Until that migration is approved, implementation stops before the Cloud identity API, runtime manifest, or Desktop preflight because those consumers must not be built on a weak hostname/database-name approximation.
