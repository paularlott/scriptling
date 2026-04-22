import scriptling.container as container

# ── Setup ─────────────────────────────────────────────────────────────────────

c = container.Client("docker")
print(f"Using driver: {c.driver()}")

# ── Volume ────────────────────────────────────────────────────────────────────

print("\n=== Volume ===")
c.volume_create("demo-data")
print("Created volume: demo-data")
print("Volumes:", c.volume_list())

# ── Pull image ────────────────────────────────────────────────────────────────

print("\n=== Pull ===")
print("Pulling ubuntu:24.04 ...")
c.image_pull("ubuntu:24.04")
print("Pull complete")

# ── Run container ─────────────────────────────────────────────────────────────
# Run a one-shot container that writes a file to the volume then exits.

print("\n=== Run ===")
id = c.run(
    "ubuntu:24.04",
    name="demo-ubuntu",
    volumes=["demo-data:/data"],
    env=["GREETING=hello from scriptling"],
    command=["/bin/bash", "-c", "echo $GREETING > /data/greeting.txt && cat /data/greeting.txt"],
)
print(f"Container ID: {id}")

# ── Inspect ───────────────────────────────────────────────────────────────────

print("\n=== Inspect ===")
info = c.inspect("demo-ubuntu")
print(f"  name:    {info['name']}")
print(f"  image:   {info['image']}")
print(f"  status:  {info['status']}")
print(f"  running: {info['running']}")

# ── List ──────────────────────────────────────────────────────────────────────

print("\n=== List (all containers) ===")
for item in c.list():
    status = "running" if item["running"] else item["status"]
    print(f"  {item['name']:<20} {status}")

# ── Stop & remove ─────────────────────────────────────────────────────────────

print("\n=== Cleanup ===")
c.stop("demo-ubuntu")
print("Stopped demo-ubuntu")
c.remove("demo-ubuntu")
print("Removed demo-ubuntu")
c.volume_remove("demo-data")
print("Removed volume demo-data")
c.image_remove("ubuntu:24.04")
print("Removed image ubuntu:24.04")

print("\nDone.")
