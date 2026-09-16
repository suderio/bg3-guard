# Contributing to BG3 Guard

Thank you for your interest in contributing! We love receiving contributions from the community. 

Please take a moment to review this document before getting started.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_Of_CONDUCT.md).

## How Can I Contribute?

### Reporting Bugs
Before opening a new issue, please search the existing issues to see if it has already been reported. If not, open a new issue and include:
* A clear, descriptive title.
* Steps to reproduce the problem.
* Expected vs. actual behavior.
* Your environment details (OS, language version, etc.).

### Suggesting Enhancements
We welcome feature ideas! Please open an issue labeled `enhancement` and describe:
* The core problem you want to solve.
* Your proposed solution or workflow.
* Why this feature would be useful to other users.

## Local Development Setup

To set up the project locally on your machine:

1. **Fork** the repository on GitHub.
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/suderio/bg3-guard
   ```
3. **Install dependencies and build**:
   ```bash
   make
   ```
4. **Create a branch** for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Pull Request Guidelines

Before submitting your Pull Request (PR), please ensure:
* All existing and new tests pass locally.
* Your code follows our established style guidelines.
* Your commit messages follow the [Conventional Commits](https://conventionalcommits.org) format.
* You link the PR to the relevant issue you are fixing.

Once submitted, a maintainer will review your code. We may ask for changes before merging.
