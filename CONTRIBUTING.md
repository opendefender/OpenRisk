# Contributing to OpenRisk

Thank you for your interest in contributing to OpenRisk! This document provides guidelines and instructions for contributing to our open-source risk management platform.

## Table of Contents

1. [Code of Conduct](#code-of-conduct)
2. [Asking Questions](#asking-questions)
3. [Providing Feedback](#providing-feedback)
4. [Reporting Bugs](#reporting-bugs)
5. [Suggesting Features](#suggesting-features)
6. [Contributing Code](#contributing-code)
7. [Pull Request Process](#pull-request-process)
8. [Development Setup](#development-setup)
9. [Testing](#testing)
10. [Documentation](#documentation)
11. [Community](#community)

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to [conduct@openrisk.io](mailto:conduct@openrisk.io).

## Asking Questions

Before opening an issue for a question, please:

1. **Check the documentation** at [https://docs.openrisk.io](https://docs.openrisk.io)
2. **Search existing issues** to see if your question has been answered
3. **Review the FAQ** section in the [README](README.md)
4. **Ask on the community forums** at [https://community.openrisk.io](https://community.openrisk.io)

For coding questions, consider asking on:
- [Stack Overflow](https://stackoverflow.com/questions/tagged/openrisk) using the `openrisk` tag
- Our [GitHub Discussions](https://github.com/opendefender/OpenRisk/discussions)
- Our [Discord Community](https://discord.gg/openrisk)

## Providing Feedback

Your comments and feedback are welcome. You can share feedback through:

1. **GitHub Issues** - For specific bugs or feature requests
2. **GitHub Discussions** - For general feedback and ideas
3. **Email** - [feedback@openrisk.io](mailto:feedback@openrisk.io)
4. **Community Chat** - Join our [Discord community](https://discord.gg/openrisk)

## Reporting Bugs

### Before Submitting a Bug Report

- **Check the FAQ** - Your issue might already be answered
- **Check the documentation** - The behavior might be expected
- **Search existing issues** - Your bug might already be reported
- **Try disabling extensions/plugins** - If applicable, to isolate the issue
- **Collect diagnostics** - Gather version info, logs, and environment details

### How to Submit a Good Bug Report

File one bug per issue. Do not enumerate multiple bugs in a single issue.

**Use a clear and descriptive title** that identifies the problem.

**Describe the exact steps to reproduce the problem**:

1. First step
2. Second step
3. Specific example to demonstrate the steps

**Provide specific examples** to demonstrate the steps. Include links to files or GitHub projects, or copy/paste snippets, which you use in those examples.

**Describe the behavior you observed** and **point out what exactly is the problem** with that behavior.

**Explain which behavior you expected to see** instead and why.

**Include screenshots and animated GIFs** if possible. You can use [this tool](https://www.cockos.com/licecap/) to record GIFs on macOS and Windows, and [this tool](https://github.com/colinkeenan/silentcast) or [this tool](https://github.com/GNOME/byzanz) on Linux.

**Include your environment**:

```
- OpenRisk Version: [e.g. 1.0.0]
- OS and Version: [e.g. macOS 13.0, Ubuntu 22.04, Windows 11]
- Browser: [e.g. Chrome, Firefox, Safari]
- Node Version: [e.g. 18.0.0]
- Go Version: [e.g. 1.25.4]
- Docker/Kubernetes: [if using containerized deployment]
```

**Include relevant logs**:

- Application logs from `/var/log/openrisk/`
- Database query logs
- API endpoint responses
- Browser console errors

### Bug Report Template

```markdown
## Summary
[Brief description of the bug]

## Steps to Reproduce
1. [First step]
2. [Second step]
3. [Expected result]

## Actual Behavior
[What actually happened]

## Expected Behavior
[What should have happened]

## Environment
- OpenRisk Version: [version]
- OS: [operating system]
- Browser: [if web UI issue]

## Logs
[Relevant logs, if available]

## Screenshots
[If applicable]
```

## Suggesting Features

### Before Submitting a Feature Request

- **Check if the feature already exists** - It might be available in a different way
- **Check existing feature requests** - Your feature might already be requested
- **Consider if this is in scope** - Features should align with OpenRisk's vision

### How to Submit a Good Feature Request

**Use a clear and descriptive title** for the feature request.

**Provide a step-by-step description of the suggested feature**:

1. Describe the current behavior
2. Explain the desired behavior
3. Describe alternatives you've considered
4. Explain why this would be useful

**Include mockups or wireframes** if the feature involves UI/UX changes.

**Provide context**:

- What problem does this feature solve?
- Who would benefit from this feature?
- How does this align with OpenRisk's vision?
- What is the impact on existing functionality?

### Feature Request Template

```markdown
## Feature Summary
[Brief description]

## Problem Statement
[Problem this solves]

## Proposed Solution
[How to implement this]

## Alternatives Considered
[Other approaches]

## Use Cases
[Real-world usage]

## Benefits
[Why this matters]
```

## Contributing Code

### Getting Started

1. **Fork the repository** - Click the "Fork" button on GitHub
2. **Clone your fork** - `git clone https://github.com/YOUR_USERNAME/OpenRisk.git`
3. **Add upstream remote** - `git remote add upstream https://github.com/opendefender/OpenRisk.git`
4. **Create a branch** - `git checkout -b fix/issue-123` or `git checkout -b feat/new-feature`

### Development Setup

For detailed setup instructions, see [DEVELOPMENT_SETUP.md](docs/DEVELOPMENT_SETUP.md).

**Quick Start**:

```bash
# Backend
cd backend
go mod download
go run cmd/server/main.go

# Frontend
cd frontend
npm install
npm start

# Database
docker run -d \
  --name openrisk-postgres \
  -e POSTGRES_PASSWORD=password \
  -p 5432:5432 \
  postgres:16

# Redis (for caching)
docker run -d \
  --name openrisk-redis \
  -p 6379:6379 \
  redis:7
```

### Code Style

- **Go**: Follow [Effective Go](https://golang.org/doc/effective_go)
- **TypeScript/React**: Follow [Google TypeScript Style Guide](https://google.github.io/styleguide/tsguide.html)
- **SQL**: Follow [SQL Style Guide](https://sqlstyle.guide/)
- **Comments**: Write clear, concise comments. Self-documenting code is preferred.
- **Naming**: Use descriptive names for variables, functions, and types.

### Commit Messages

- **Use clear, descriptive titles** (max 72 characters)
- **Reference related issues** using `#issue_number`
- **Use conventional commits** when possible:
  - `feat:` for new features
  - `fix:` for bug fixes
  - `docs:` for documentation
  - `refactor:` for code refactoring
  - `test:` for tests
  - `chore:` for maintenance

Example:
```
feat: Add organization management system

- Implement multi-org support with RBAC
- Add subscription tier management
- Include team collaboration features

Fixes #123
```

## Pull Request Process

### Before Submitting

1. **Check for existing pull requests** - Search for related PRs
2. **Run tests locally** - Ensure all tests pass
3. **Test your changes** - Verify the fix/feature works
4. **Update documentation** - Include any necessary docs updates
5. **Sync with upstream** - `git pull upstream main`

### Submitting a Pull Request

1. **Push to your fork** - `git push origin your-branch-name`
2. **Create a pull request** - Use the GitHub PR template
3. **Fill out the PR template** - Provide all required information
4. **Link related issues** - Use `Fixes #123` or `Relates to #456`
5. **Request review** - Tag relevant maintainers

### PR Title Format

```
[TYPE] Brief description

Types:
- [FEATURE] - New feature
- [BUG] - Bug fix
- [REFACTOR] - Code refactoring
- [DOCS] - Documentation
- [TEST] - Tests
- [PERF] - Performance improvement
```

### PR Description Template

```markdown
## Description
[Brief summary of changes]

## Type of Change
- [ ] Bug fix (non-breaking change)
- [ ] New feature (non-breaking change)
- [ ] Breaking change
- [ ] Documentation update
- [ ] Performance improvement

## Related Issue
Fixes #[issue_number]

## Changes Made
- [Change 1]
- [Change 2]
- [Change 3]

## Testing
- [ ] Added tests
- [ ] Updated tests
- [ ] All tests passing

## Checklist
- [ ] Code follows style guidelines
- [ ] Documentation updated
- [ ] No breaking changes
- [ ] PR title is descriptive
- [ ] Related issues linked

## Screenshots/Videos
[If applicable]
```

### Review Process

1. **Automated checks** - CI/CD pipeline must pass
2. **Code review** - At least one maintainer review required
3. **Address feedback** - Make requested changes
4. **Final approval** - Maintainer approves and merges

### After Merge

- Your changes are tested in staging
- Changes are deployed to production after validation
- You're added to the contributors list

## Testing

### Running Tests

```bash
# Backend tests
cd backend
go test ./...

# Frontend tests
cd frontend
npm test

# E2E tests
npm run test:e2e

# Load testing
k6 run tests/performance/load_test.js
```

### Writing Tests

- **Unit tests** - Test individual functions
- **Integration tests** - Test multiple components together
- **E2E tests** - Test complete user workflows
- **Performance tests** - Test at scale

Example:
```go
func TestCreateRisk(t *testing.T) {
    // Arrange
    req := &CreateRiskRequest{
        Name: "Test Risk",
    }
    
    // Act
    result, err := service.CreateRisk(context.Background(), req)
    
    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, "Test Risk", result.Name)
}
```

## Documentation

### Writing Documentation

- **Be clear and concise** - Use simple language
- **Include examples** - Show how to use features
- **Update TOC** - Keep table of contents current
- **Link related docs** - Help users navigate

### Documentation Locations

- **User Docs** - `/docs` directory
- **API Docs** - OpenAPI/Swagger specs
- **Code Comments** - Inline documentation
- **README** - Quick start and overview
- **CHANGELOG** - Document changes

### Updating Documentation

1. Make changes to `.md` files
2. Test links and formatting
3. Run spell check
4. Submit in same PR as code changes

## Community

### Communication Channels

- **GitHub Issues** - Bug reports and feature requests
- **GitHub Discussions** - Questions and ideas
- **Discord** - [Join our community](https://discord.gg/openrisk)
- **Email** - [community@openrisk.io](mailto:community@openrisk.io)
- **Twitter** - [@OpenRiskIO](https://twitter.com/OpenRiskIO)

### Maintainers

Current maintainers:

- **@alex-dembele** - Project Lead
- **@team-openrisk** - Core Team

### Getting Help

- Review [documentation](https://docs.openrisk.io)
- Check [FAQ](FAQ.md)
- Ask in [discussions](https://github.com/opendefender/OpenRisk/discussions)
- Join [Discord](https://discord.gg/openrisk)

## Recognition

Contributors are recognized in:

1. **CONTRIBUTORS.md** - All contributors listed
2. **GitHub** - Automatic contributor tracking
3. **Releases** - Contributors mentioned in release notes
4. **Discord** - Special roles for active contributors

## License

By contributing to OpenRisk, you agree that your contributions will be licensed under the [license](LICENSE) of the project.

## Additional Notes

- **Security Issues** - See [SECURITY.md](SECURITY.md) for reporting
- **Code of Conduct** - See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- **License** - See [LICENSE](LICENSE)

## Thank You

Thank you for contributing to OpenRisk! Your efforts help make this project better for everyone.

---

**Questions?** Feel free to:
- Open a discussion at [GitHub Discussions](https://github.com/opendefender/OpenRisk/discussions)
- Email us at [contributors@openrisk.io](mailto:contributors@openrisk.io)
- Join our [Discord community](https://discord.gg/openrisk)

**Last Updated**: March 2, 2026  
**Version**: 1.0
