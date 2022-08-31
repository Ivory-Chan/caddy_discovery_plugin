#!/bin/sh

# USAGE: go run -exec ./setcap.sh <args...>

setcap cap_net_bind_service=+ep "$1"
"$@"
