# Serve a static site with nginx. Run it as a user who can sudo:
#
#   scriptling web_server.py
#
# Run it again: nothing changes unless something has drifted.

import scriptling.provision.command as command
import scriptling.provision.file as file
import scriptling.provision.package as package

SITE = "/srv/hello"
CONF = """server {
    listen 8080;
    root /srv/hello;
}
"""

print("nginx:", package.ensure("nginx"))
print("site:", file.ensure_directory(SITE, owner="www-data", group="www-data", become=True))
print("page:", file.ensure(SITE + "/index.html", "<h1>Hello from scriptling</h1>\n",
                           owner="www-data", group="www-data", become=True))

status = file.ensure("/etc/nginx/conf.d/hello.conf", CONF, become=True)
print("config:", status)
if status != file.UNCHANGED:
    command.run(["nginx", "-t"], become=True)  # a bad config stops here, with nginx's reason
    if command.run("systemctl is-active --quiet nginx", check=False).ok:
        command.run(["systemctl", "reload", "nginx"], become=True)
        print("nginx reloaded")
