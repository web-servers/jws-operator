# Contributing to JWS Operator

Thanks for your interest in contributing to `jws-operator`.

This guide focuses on the contributor workflow for this repository. For product-level usage and deployment details, see `README.md`.

## Prerequisites

Required:

- Go (see `go.mod` for version used by the project)
- `make`
- `kubectl`

Depending on your task:

- `docker` or `podman` (for image builds)
- `oc` (for OpenShift-specific workflows and e2e setup)
- `kind` (for `make test-e2e`)

Most development tools used by the build (`controller-gen`, `setup-envtest`, `golangci-lint`, `kustomize`) are bootstrapped by `make` targets into `bin/`.

## Local Setup

```bash
git clone https://github.com/web-servers/jws-operator.git
cd jws-operator
make help
```

`make help` shows the supported targets and is the best entry point before running other commands.

## Build

Build the manager binary:

```bash
make build
```

This generates manifests/code, runs `fmt` and `vet`, and builds `bin/manager`.

Build and push an image:

```bash
export IMG=quay.io/<your-user>/jws-operator:latest
make docker-build docker-push
```

If you use Podman:

```bash
make docker-build docker-push CONTAINER_TOOL=podman
```

## Tests

### Unit tests

```bash
make test
```

This target sets up envtest assets automatically (`make setup-envtest`) and runs all non-e2e Go tests.

### E2E tests on Kind

```bash
make test-e2e
```

This creates a Kind cluster (if missing), deploys the operator, runs e2e tests, then cleans up.

### E2E tests on a real cluster

```bash
make test-e2e-real
```

Common variables:

- `NAMESPACE_FOR_TESTING` (default: `jws-operator-tests`)
- `TEST_IMG` (default: `quay.io/web-servers/tomcat10:latest`)
- `EXECUTE_TEST` (default: `WebServerControllerTest`)
- `TEST_PARAM` (extra ginkgo flags)

Example:

```bash
make test-e2e-real EXECUTE_TEST=BasicTest
```

Real-cluster prerequisites (secrets/storage) are documented in `README.md` and `test-scripts/README.md`.

## Linting and Formatting

```bash
make lint
make lint-fix
make lint-config
```

For formatting only:

```bash
make fmt
```

## Pull Request Workflow

1. Fork the repository and create a branch from `main`.
2. Make focused changes with clear commit messages.
3. Run relevant checks locally (`make build`, `make lint`, `make test`, and e2e where applicable).
4. Open a PR against `web-servers/jws-operator:main`.
5. Link related issues in the PR description (for example, `Fixes #123`).

## Reporting Issues and Getting Help

- Use [GitHub Issues](https://github.com/web-servers/jws-operator/issues) for bugs and feature requests.
- For project context and runtime usage, refer to `README.md`.
