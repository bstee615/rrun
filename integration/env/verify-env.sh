#!/bin/sh
set -eu

[ "$SIMPLE" = "ok" ]
[ "$WITH_SPACES" = "hello world" ]
[ "$WITH_QUOTE" = "it's complex" ]
[ "$WITH_EQUALS" = "a=b" ]
[ -z "${REMOVE_ME+x}" ]

printf '%s\n' "$SIMPLE"
printf '%s\n' "$WITH_SPACES"
printf '%s\n' "$WITH_QUOTE"
printf '%s\n' "$WITH_EQUALS"
printf '%s\n' "unset"
