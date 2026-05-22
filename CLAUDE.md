### Strict instructions:
The application must follow programming best practices and go-specific best practices. The is to create a well maintained and maintainable, testable, and expandable application.

The application should rely on external dependencies minimally. If there is a good fit that would save a lot of trouble, ask for approval to use an external dependency. Approved external dependencies are:
- `github.com/spf13/cobra`
- `github.com/lfrysta/rptgen`

Before committing any code:
- All test must pass
- The code must be formatted with `go fmt``
- The go linter must run without errors: `golangci-lint run`

When commiting code:
- Never commit to main
- Always create a dedicated, appropriately names branch for the work
- Always start from the main branch that is up to date. Report problems if any are encountered.
- Create appropriate, detailed commit messages in the format: subject + message

When opening pull requests:
- Always open pull requests to the main branch unless otherwise instructed.
- Create appropriate, detailed message as to what changed in the pull request.
