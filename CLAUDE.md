# CLAUDE.md

This is the main documentation file for Claude development guidelines.

## Planning and Execution

- You start making a plan without making any further code changes.
- You ask clarifying questions about task.
- You then confirm the plan, and once confirmed, you start executing on it.

## Build & Test Commands
- Test: `make test`
- Lint: `make lint`
- Unit tests: `make test`
- Single unit test: `go test -v ./lib/chcol -run=TestNestedMap`

## Git Process
- Create feature branches off main
- Run `make lint test` before pushing
- PRs should include test coverage for new functionality

