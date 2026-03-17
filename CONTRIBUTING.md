# Contributing to JWS Operator

Thank you for your interest in contributing to the JBoss Web Server (JWS) Operator! This guide will help you get started with setting up your development environment, running tests, and submitting contributions.

---

## Contents

- [Prerequisites](#prerequisites)
- [Repository Structure](#repository-structure)
- [Local Development Setup](#local-development-setup)
- [Building the Operator](#building-the-operator)
- [Running Tests](#running-tests)
  - [Unit Tests](#unit-tests)
  - [End-to-End Tests on a Real Cluster](#end-to-end-tests-on-a-real-cluster)
- [Code Style and Linting](#code-style-and-linting)
- [Submitting a Pull Request](#submitting-a-pull-request)
- [Issue Tracker](#issue-tracker)
- [Getting Help](#getting-help)

---

## Prerequisites

Before you begin, ensure the following tools are installed:

| Tool | Purpose | Install Guide |
|------|---------|---------------|
| **Go 1.24+** | Primary development language | [golang.org/doc/install](https://golang.org/doc/install) |
| **kubectl** | Kubernetes CLI | [kubernetes.io/docs/tasks/tools](https://kubernetes.io/docs/tasks/tools/) |
| **oc** (optional) | OpenShift CLI (required for OpenShift-specific features) | [docs.openshift.com](https://docs.openshift.com/container-platform/latest/cli_reference/openshift_cli/getting-started-cli.html) |
| **podman** or **docker** | Container image building | [podman.io](https://podman.io/getting-started/installation) |
| **operator-sdk** (optional) | Operator scaffolding and bundle management | [sdk.operatorframework.io](https://sdk.operatorframework.io/docs/installation/) |
| **make** | Build automation | Usually pre-installed on Linux/macOS |

You will also need the following for running unit tests locally:

- **kube-apiserver**, **etcd**, and **kubectl** - required by the envtest framework. These are downloaded automatically via `make setup-envtest`. For details, see: [kubebuilder.io/reference/envtest](https://book.kubebuilder.io/reference/envtest.html)

---

## Repository Structure

```
jws-operator/
├── api/v1alpha1/             # CRD type definitions (WebServer, WebServerSpec, etc.)
├── cmd/main.go               # Operator entrypoint
├── internal/
│   ├── controller/           # Reconciliation logic, helpers, and templates
│   └── webhook/v1alpha1/     # Admission webhook validation
├── config/                   # Kustomize manifests (CRDs, RBAC, manager, samples)
├── test/
│   ├── e2e/                  # End-to-end test suite (Ginkgo)
│   └── utils/                # Test utilities
├── test-scripts/             # Scripts for cluster test setup (NFS, TLS, etc.)
├── hack/                     # Code generation boilerplate
├── Dockerfile                # Multi-stage build for the operator image
├── Makefile                  # All build, test, and deploy targets
└── .golangci.yml             # Linter configuration
```

### Key Source Files

- **`api/v1alpha1/webserver_types.go`** - CRD type definitions for the `WebServer` resource
- **`internal/controller/webserver_controller.go`** - Main reconciliation loop
- **`internal/controller/helper.go`** - Helper functions (CRUD operations, hash calculation, etc.)
- **`internal/controller/templates.go`** - Kubernetes resource generators (Deployments, StatefulSets, Routes, etc.)
- **`internal/controller/service.go`** - Prometheus service management
- **`internal/webhook/v1alpha1/webserver_webhook.go`** - Admission webhook for input validation

---

## Local Development Setup

1. **Fork and clone the repository**:

   ```bash
   git clone https://github.com/<your-username>/jws-operator.git
   cd jws-operator
   ```

2. **Download Go dependencies**:

   ```bash
   go mod download
   ```

3. **Generate CRDs and deepcopy methods** (required after modifying API types):

   ```bash
   make manifests generate
   ```

4. **Build the operator binary**:

   ```bash
   make build
   ```

   The binary will be placed at `bin/manager`.

---

## Building the Operator

### Build the binary locally

```bash
make build
```

### Build and push the container image

```bash
export IMG=quay.io/<your-username>/jws-operator:latest
podman login quay.io
make manifests docker-build docker-push
```

> **Note:** The default container tool is `docker`. To use `podman`, set `CONTAINER_TOOL=podman` or ensure `podman-docker` is installed.

### Generate the vendor directory

If you need the `vendor` directory (e.g., for internal build systems):

```bash
go mod vendor
```

---

## Running Tests

### Unit Tests

Unit tests use the [envtest](https://book.kubebuilder.io/reference/envtest.html) framework, which runs a lightweight Kubernetes API server and etcd locally.

**One-time setup** - download the envtest binaries:

```bash
make setup-envtest
```

**Run all unit tests**:

```bash
make test
```

This will:
1. Generate manifests and code
2. Run `go fmt` and `go vet`
3. Execute all tests (excluding e2e) with coverage output to `cover.out`

### End-to-End Tests on a Real Cluster

E2E tests require a running Kubernetes or OpenShift cluster with the operator installed.

**Prerequisites**:

1. A running cluster (Kubernetes or OpenShift)
2. The operator installed and available in the test namespace
3. A Docker config secret for pulling images:

   ```bash
   kubectl create secret generic secretfortests \
     --from-file=.dockerconfigjson=$HOME/.docker/config.json \
     --type=kubernetes.io/dockerconfigjson
   ```

4. For TLS route tests, a TLS secret:

   ```bash
   kubectl create secret generic test-tls-secret \
     --from-file=server.crt=server.crt \
     --from-file=server.key=server.key \
     --from-file=ca.crt=ca.crt
   ```

5. For PersistentLogs tests, set up PVs and StorageClass. See [test-scripts/README.md](test-scripts/README.md) for details.

**Run the e2e test suite**:

```bash
make test-e2e-real
```

**Run a specific test**:

```bash
make test-e2e-real EXECUTE_TEST='BasicTest'
```

The `EXECUTE_TEST` parameter supports Ginkgo `Context` or `It` names.

**Configuration environment variables**:

| Variable | Description | Default |
|----------|-------------|---------|
| `NAMESPACE_FOR_TESTING` | Namespace for test WebServer deployments | `jws-operator-tests` |
| `TEST_IMG` | Default image for tests | `quay.io/web-servers/tomcat10:latest` |
| `EXECUTE_TEST` | Comma-separated list of tests to run | `WebServerControllerTest` |
| `TEST_PARAM` | Additional Ginkgo parameters | - |

> **Note:** The full e2e test suite takes approximately 40 minutes.

---

## Code Style and Linting

This project uses [golangci-lint](https://golangci-lint.run/) (v2) for code quality checks. The configuration is in [`.golangci.yml`](.golangci.yml).

### Run the linter

```bash
make lint
```

### Auto-fix linting issues

```bash
make lint-fix
```

### Verify the linter configuration

```bash
make lint-config
```

### Key linting rules

The project enforces the following linters (among others):

- **`errcheck`** - Check for unchecked errors
- **`govet`** - Report suspicious constructs
- **`staticcheck`** - Advanced static analysis
- **`misspell`** - Catch common spelling mistakes
- **`revive`** - Opinionated Go linter (comment spacing, import shadowing)
- **`gocyclo`** - Cyclomatic complexity checks
- **`ginkgolinter`** - Ginkgo-specific best practices

### Formatting

The project uses `gofmt` and `goimports` for code formatting:

```bash
make fmt
```

---

## Submitting a Pull Request

### Before submitting

1. **Fork the repository** on GitHub
2. **Create a feature branch** from `main`:

   ```bash
   git checkout -b fix/your-descriptive-branch-name
   ```

3. **Make your changes** and commit with clear, descriptive messages:

   ```bash
   git commit -m "fix: brief description of the change"
   ```

4. **Ensure your changes build and pass tests**:

   ```bash
   go build ./...
   go vet ./...
   make lint
   make test    # If you have envtest binaries set up
   ```

5. **Push your branch** and open a PR against the upstream `main` branch

### PR guidelines

- **Keep PRs focused** - One logical change per PR (one bug fix, one feature, one docs improvement)
- **Write a clear description** - Explain what you changed and why
- **Link related issues** - Use `Fixes #123` or `Refs #123` in the PR description
- **Ensure CI passes** - The project runs linting and unit tests on every PR via GitHub Actions
- **Be responsive to reviews** - Maintainers may request changes; this is normal and part of the collaborative process

### Commit message conventions

Use conventional commit prefixes when appropriate:

- `fix:` - Bug fixes
- `feat:` - New features
- `docs:` - Documentation changes
- `test:` - Test additions or modifications
- `refactor:` - Code refactoring without functional changes
- `chore:` - Build/tooling changes

---

## Issue Tracker

The project uses [GitHub Issues](https://github.com/web-servers/jws-operator/issues) for bug reports and feature requests. Issue templates are available for:

- **Bug reports** - `.github/ISSUE_TEMPLATE/bug_report.md`
- **Feature requests** - `.github/ISSUE_TEMPLATE/feature_request.md`

Before opening a new issue, please search existing issues to avoid duplicates.

---

## Getting Help

- **GitHub Issues** - For bug reports, feature requests, and technical questions
- **Apache Tomcat Mailing Lists** - For Tomcat configuration and support questions:
  - [users@tomcat.apache.org](https://tomcat.apache.org/lists.html) - Configuration and support
  - [dev@tomcat.apache.org](https://tomcat.apache.org/lists.html) - Development discussions
- **Upstream Repositories** - Related projects under the [web-servers GitHub org](https://github.com/web-servers)
