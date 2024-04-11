#!/bin/sh
set -e
COURSE_DEFINITION=/mnt/v/GitHub/_others/docker-tester/internal/test_helpers/course_definition.yml
export CODECRAFTERS_SUBMISSION_DIR=/mnt/v/GitHub/feliposz/build-your-own-docker-go
export CODECRAFTERS_TEST_CASES_JSON=$(yq -I0 '[.stages[] | {"slug": .slug, "tester_log_prefix": .slug, "title": (path | .[-1] + 1) + ") " + .name }]' -o=json $COURSE_DEFINITION)
go build -buildvcs="false" -o mydocker ./app/main.go
/mnt/v/GitHub/_others/docker-tester/docker-tester
