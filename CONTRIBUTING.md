# Contributing to Minato

First off, thank you for considering contributing to Minato! It's people like you that make open-source software such a great community.

## Development Workflow

Minato uses a Trunk-Based Development model with Pull Requests. The `main` branch is protected and always represents the latest tested, working code.

Here is the general workflow for submitting changes:

### 1. Local Setup

Make sure you have Go installed (1.23 or newer).

```bash
# Clone the repository
git clone https://github.com/dwikynator/minato.git
cd minato

# Download dependencies
go mod download

# Verify everything works locally
go test -v -race ./...
```

### 2. Making Changes

1.  **Branch off** from the latest `main` branch:
    ```bash
    git checkout main
    git pull origin main
    git checkout -b feature/your-feature-name
    ```
2.  **Write your code.**
3.  **Ensure tests pass.** If you add a new feature, please add tests for it.
    ```bash
    go test -v -race ./...
    ```
4.  **Commit your changes.** We prefer atomic commits with descriptive messages.
    ```bash
    git commit -m "feat(middleware): add rate limiting options"
    ```

### 3. Submitting a Pull Request

1.  Push your branch up to GitHub:
    ```bash
    git push origin feature/your-feature-name
    ```
2.  Open a Pull Request on GitHub against the `main` branch.
3.  Fill out the PR description explaining *what* changed and *why*.
4.  Our GitHub Actions CI will automatically run all tests against your PR.
    *   **Wait for the green checkmark.** If the CI fails, look at the logs, fix the issue locally, and push again.
5.  A maintainer will review your code. Once approved and checks pass, it will be merged into `main`!

## Code Style

- Follow standard Go formatting guidelines. Run `go fmt ./...` before committing.
- Ensure all exported symbols have descriptive Godoc comments as per Step B1 of our development guide.
- Keep middleware modular and independent.
