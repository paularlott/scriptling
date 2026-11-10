# Install a few tools and set up your shell. On Linux run it as a user who can
# sudo; on macOS it uses Homebrew, which needs no root:
#
#   scriptling dev_tools.py

import scriptling.provision.file as file
import scriptling.provision.package as package

print("package manager:", package.manager())
print("tools:", package.ensure(["git", "jq", "ripgrep"]))
print("shell:", file.ensure_block("~/.bashrc", "export EDITOR=vim\nalias ll='ls -la'", id="dev-tools"))
