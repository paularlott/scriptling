# Provisioning

Set up a machine from a script: install packages, write config files, run commands. Every step checks first and changes only what needs changing, so the scripts are safe to run again.

| Script | What it does |
|--------|--------------|
| [web_server.py](web_server.py) | Installs nginx and serves a static site on port 8080 |
| [dev_tools.py](dev_tools.py) | Installs git, jq and ripgrep, and adds settings to `~/.bashrc` |

Run them as yourself; scriptling uses sudo for the steps that need root:

```bash
scriptling web_server.py
```

## Running as root

Steps that need root say so with `become=True`; everything else runs as you.

```python
file.ensure("/etc/nginx/conf.d/hello.conf", CONF, become=True)
command.run(["systemctl", "reload", "nginx"], become=True)
```

- sudo does the work, so the user running the script must be allowed to sudo.
- `--deny-become` refuses these steps, and so does disabling the subprocess library; a refused step stops the script.
- sudo is asked once per script. Where sudo needs no password, nothing is asked. With no terminal to ask on, the step fails straight away.
- `package.ensure()` uses root by itself when the package manager needs it, so it takes no `become`.
- New files written as root belong to root unless you pass `owner=` and `group=`.
- Already root (`sudo scriptling web_server.py`)? The steps just run.
