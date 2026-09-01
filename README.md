# Gooo OpenTofu Change Contract

This repository is a read-only, deterministic fixture consumer. It combines a
pinned OpenTofu plan JSON document, a pinned OpenAPI document, and an explicit
Gooo-owned resource-to-service mapping into a change contract. It never runs
OpenTofu or Terraform, installs providers, calls a cloud, or mutates input
repositories.

The implementation is introduced by the feature pull request. The `main`
branch bootstrap intentionally contains only repository metadata.

