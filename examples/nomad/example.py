import os
import scriptling.nomad as nomad

# ── Setup ─────────────────────────────────────────────────────────────────────
# Point at your Nomad cluster. NOMAD_ADDR / NOMAD_TOKEN are read here rather
# than by the library itself, so any env var names work.

addr = os.environ.get("NOMAD_ADDR", "http://127.0.0.1:4646")
token = os.environ.get("NOMAD_TOKEN", "")

c = nomad.Client(addr, token=token)
print(f"Using Nomad at: {addr}")

# ── Jobs ──────────────────────────────────────────────────────────────────────
# List running jobs across all namespaces.

print("\n=== Jobs ===")
jobs = c.jobs_list(namespace="*")
if not jobs:
    print("  (none found)")
for j in jobs:
    print(f"  {j['id']:<30} ns={j['namespace']:<12} type={j['type']:<8} status={j['status']}")

# ── CSI Volumes ───────────────────────────────────────────────────────────────
# List CSI volumes across all namespaces.

print("\n=== CSI Volumes ===")
volumes = c.csi_volumes_list(namespace="*")
if not volumes:
    print("  (none found)")
for v in volumes:
    print(f"  {v['id']:<30} plugin={v['plugin_id']:<20} schedulable={v['schedulable']} namespace={v['namespace']}")

print("\nDone.")
