#!/usr/bin/env bash
set -euo pipefail

while IFS= read -r line; do
  method=$(printf '%s\n' "$line" | jq -r '.method')
  id=$(printf '%s\n' "$line" | jq -r '.id')

  case "$method" in
    scriptling.handshake)
      printf '{"jsonrpc":"2.0","id":%s,"result":{"protocol":"1.0","transport":"json","library":{"name":"hello","version":"1.0.0","description":"Bash hello plugin"},"capabilities":[],"schema":{"functions":[{"name":"greet","args":["name"],"wrapper":"generated"}],"classes":[],"constants":[]}}}\n' "$id"
      ;;
    function.call)
      name=$(printf '%s\n' "$line" | jq -r '.params.name')
      if [ "$name" = "greet" ]; then
        who=$(printf '%s\n' "$line" | jq -r '.params.args[0].value')
        jq -nc --argjson id "$id" --arg text "Hello, $who" \
          '{"jsonrpc":"2.0","id":$id,"result":{"type":"string","value":$text}}'
      else
        jq -nc --argjson id "$id" \
          '{"jsonrpc":"2.0","id":$id,"error":{"code":-32601,"message":"unknown function"}}'
      fi
      ;;
    plugin.shutdown)
      printf '{"jsonrpc":"2.0","id":%s,"result":null}\n' "$id"
      exit 0
      ;;
    *)
      jq -nc --argjson id "$id" \
        '{"jsonrpc":"2.0","id":$id,"error":{"code":-32601,"message":"unknown method"}}'
      ;;
  esac
done
