# Build Your Own Docker

This is my solution in Go for the
["Build Your Own Docker" Challenge](https://codecrafters.io/challenges/docker).

**Note**: Head over to
[codecrafters.io](https://codecrafters.io) to try the challenge yourself.

# Running tests locally

To run tests locally (linux only!), the following need to be built and installed accordingly:

- [docker-tester](https://github.com/codecrafters-io/docker-tester/)
- [docker-explorer](https://github.com/codecrafters-io/docker-explorer)

# To do

- [x] Execute a program
- [x] Wireup stdout & stderr
- [x] Handle exit codes
- [x] Filesystem isolation
- [x] Process isolation
- [x] Fetch an image from the Docker Registry
- [x] Passing all stages from [docker-tester](https://github.com/codecrafters-io/docker-tester/)
- [ ] Implement the missing features from [Coding Challenges](https://codingchallenges.fyi/challenges/challenge-docker/)
- [ ] Explorer more about `namespaces`
- [ ] Properly mount `/dev`, `/proc` (?!?!)
- [ ] Apply the config from the manifest (e.g. hostname, environment variables, etc.)
