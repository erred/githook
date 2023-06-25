# post-receive-buildkite

A binary to be used as a git `post-receive` hook for triggering builds on buildkite

## Config

Expects the following configuration:

Environment:

- `BUILDKITE_ORG_SLUG`: buildkite organization name
- `BUILDKITE_API_TOKEN`: an api token with builds write for the organization

## Usage

Place in hooks as `post-receive`

Send the `ci.skip` push option to do nothing.
