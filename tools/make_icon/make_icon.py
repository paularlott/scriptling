# Generate the [[icons]] TOML block for a tool from an image file.
#
#   scriptling tools/make_icon/make_icon.py icon.svg
#   scriptling tools/make_icon/make_icon.py logo.png -- >> mytool.toml
#
# Part of the scriptling toolchain: works for any consumer of the MCP icons
# convention (scriptling server, llmrouter, fortix-mcp).
#
# Reads the file, base64-encodes it into a data: URI (the icons convention
# accepts https:// URLs and data: URIs), infers the mime type from the
# extension, and prints the block ready to paste into the tool's .toml.

import sys
import os
import base64

MIME_BY_EXT = {
    ".svg": "image/svg+xml",
    ".png": "image/png",
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".gif": "image/gif",
    ".webp": "image/webp",
    ".ico": "image/x-icon",
}

args = sys.argv[1:]
if len(args) < 1 or args[0] in ("-h", "--help"):
    print("usage: scriptling tools/make_icon/make_icon.py <icon.svg|png|jpg|gif|webp|ico>")
    sys.exit(1)

path_in = args[0]
ext = os.path.splitext(path_in)[1].lower()
mime = MIME_BY_EXT.get(ext)
if mime is None:
    print("error: unsupported extension " + ext + " (supported: " + ", ".join(sorted(MIME_BY_EXT)) + ")")
    sys.exit(1)

import os

content = os.read_bytes(path_in)

if len(content) == 0:
    print("error: " + path_in + " is empty")
    sys.exit(1)

encoded = base64.b64encode(content)
sizes = 'sizes = ["any"]' if ext == ".svg" else ""

print("")
print("[[icons]]")
print('src = "data:' + mime + ';base64,' + encoded + '"')
print('mimeType = "' + mime + '"')
if sizes:
    print(sizes)
