#!/usr/bin/env bash
set -euo pipefail

# Bash plugin: the classic greet surface, plus a fetcher serving bsh://
# sources. Everything the protocol needs is jq, printf and base64 — proof of
# how small the plugin contract is.


bsh_hi='def greeting(name):
    return "hello from bsh://libs, " + name
'

bsh_hello_script='#!/usr/bin/env scriptling
import hi
import sys
print(hi.greeting(sys.argv[1] if len(sys.argv) > 1 else "World"))
'

send_result() { # id, result-json
  jq -nc --argjson id "$1" --argjson result "$2" \
    '{"jsonrpc":"2.0","id":$id,"result":$result}'
}

send_error() { # id, code, message
  jq -nc --argjson id "$1" --argjson code "$2" --arg msg "$3" \
    '{"jsonrpc":"2.0","id":$id,"error":{"code":$code,"message":$msg}}'
}

b64() { printf '%s' "$1" | base64 | tr -d '\n'; }

while IFS= read -r line; do
  method=$(printf '%s\n' "$line" | jq -r '.method')
  id=$(printf '%s\n' "$line" | jq -r '.id')

  case "$method" in
    scriptling.handshake)
      printf '{"jsonrpc":"2.0","id":%s,"result":{"protocol":"1.0","transport":"json","library":{"name":"hello","version":"1.0.0","description":"Bash hello plugin with a bsh:// fetcher"},"capabilities":["fetch"],"scheme":"bsh","schema":{"functions":[{"name":"greet","args":["name"],"wrapper":"generated"}],"classes":[],"constants":[]}}}\n' "$id"
      ;;
    function.call)
      name=$(printf '%s\n' "$line" | jq -r '.params.name')
      if [ "$name" = "greet" ]; then
        who=$(printf '%s\n' "$line" | jq -r '.params.args[0].value')
        jq -nc --argjson id "$id" --arg text "Hello, $who" \
          '{"jsonrpc":"2.0","id":$id,"result":{"type":"string","value":$text}}'
      else
        send_error "$id" -32601 "unknown function"
      fi
      ;;
    fetch.read)
      source=$(printf '%s\n' "$line" | jq -r '.params.source')
      path=$(printf '%s\n' "$line" | jq -r '.params.path // ""')

      content=""
      case "$path" in
        "")    [ "$source" = "bsh://scripts/hello" ] && content=$bsh_hello_script ;;
        lib/hi.py)      content=$bsh_hi ;;
      esac

      # The host caches nothing it fetches, so return content plainly with no
      # etag. A fetcher whose backend is slow enough to cache would return an
      # etag and answer {"not_modified":true} when the request's etag matches.
      if [ -z "$content" ]; then
        send_error "$id" -32001 "fetch source not found: $path in $source"
      else
        send_result "$id" '{"data":"'"$(b64 "$content")"'"}'
      fi
      ;;
    fetch.list)
      source=$(printf '%s\n' "$line" | jq -r '.params.source')
      path=$(printf '%s\n' "$line" | jq -r '.params.path // ""')
      case "$source" in
        bsh://libs)
          case "$path" in
            ""|".")     send_result "$id" '{"entries":[{"name":"lib","is_dir":true}]}' ;;
            lib)        send_result "$id" '{"entries":[{"name":"hi.py","is_dir":false}]}' ;;
            *)          send_error "$id" -32001 "fetch source not found: $path in $source" ;;
          esac
          ;;
        *)
          send_error "$id" -32001 "fetch source not found: $source"
          ;;
      esac
      ;;
    plugin.shutdown)
      printf '{"jsonrpc":"2.0","id":%s,"result":null}\n' "$id"
      exit 0
      ;;
    *)
      send_error "$id" -32601 "unknown method"
      ;;
  esac
done
