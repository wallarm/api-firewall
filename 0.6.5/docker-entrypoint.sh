#!/bin/sh
set -e

# this if will check if the first argument is a flag
# but only works if all arguments require a hyphenated flag
# -v; -SL; -f arg; etc will work, but not arg1 arg2
if [ "$#" -eq 0 ] || [ "${1#-}" != "$1" ]; then
    set -- api-firewall "$@"
fi

if [ "$1" = 'api-firewall' ]; then
	shift # "api-firewall"
	set -- api-firewall "$@"
fi

exec "$@"
