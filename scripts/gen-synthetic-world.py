#!/usr/bin/env python3
"""生成开发用种子数据（synthetic-world.json）。

真实库有 131 节点，但含个人信息与生产地址，不能进仓库。这里按真实数据的
**结构形态**造一份等价规模的替身：

  - 18 台 Machine -> 8 台
  - 74 个 SoftwareInstance -> 20 个（约每台主机 2~3 个）
  - 22 个 AccountCredential -> 6 个
  - 全部 id 是固定 UUID，重新生成不会改变，便于反复导入

所有名称、地址、路径都是编造的：域名用 example.test，网段用 RFC 5737
规定的文档专用地址，凭据只指向 synthetic:// 占位引用，不含任何真实值。

用法：python3 scripts/gen-synthetic-world.py > internal/fixtures/synthetic-world.json
"""
import json
import uuid

NAMESPACE = uuid.UUID("6f1c2b1e-8a4d-4c3e-9a51-7d2f0c8b4e11")


def uid(label):
    """稳定 UUID：同一 label 永远得到同一个 id。"""
    return str(uuid.uuid5(NAMESPACE, label))


SOURCE = "synthetic test fixture"
EVIDENCE = "observed"
CERTAINTY = "confirmed"


def node(label, type_, name, lifecycle="active", **details):
    n = {
        "id": uid(label),
        "type": type_,
        "name": name,
        "lifecycle": lifecycle,
        "source": SOURCE,
        "evidenceKind": EVIDENCE,
        "certainty": CERTAINTY,
        "modelVersion": 1,
    }
    for key, value in details.items():
        if value is not None:
            n[key] = value
    return n


nodes = []
relations = []


def relate(from_label, predicate, to_label):
    relations.append({
        "id": uid(f"rel:{from_label}:{predicate}:{to_label}"),
        "from": uid(from_label),
        "predicate": predicate,
        "to": uid(to_label),
        "source": SOURCE,
        "evidenceKind": EVIDENCE,
        "certainty": CERTAINTY,
    })


def link(label, predicate, spec):
    """在 Self 上挂一个节点。spec 是 (type, name, lifecycle) 三元组。"""
    nodes.append(node(label, *spec))
    relate("self", predicate, label)


# --- Self ---------------------------------------------------------------
nodes.append(node("self", "Self", "Self", lifecycle=None))

# --- 硬件（3 台）--------------------------------------------------------
for label, name, category, model, location in [
    ("hw-laptop", "Sample laptop", "laptop", "Example Book 14", "mobile"),
    ("hw-tower", "Sample tower", "desktop", "Example Tower 7", "fixed@home"),
    ("hw-board", "Sample single-board computer", "sbc", "Example Pi 5", "fixed@home"),
]:
    link(label, "knowsAbout", (
        "Hardware", name, "active",
    ))
    nodes[-1]["hardware"] = {
        "category": category,
        "model": model,
        "location": location,
        "specifications": {"cpu": "8 cores", "memory": "16 GiB"},
        "status": "active",
        "accessReference": f"synthetic://access/{label}",
        "maintenanceReference": f"synthetic://maintenance/{label}",
    }

# --- 机器（8 台）--------------------------------------------------------
# (label, name, parent_hw, hostname, ip, os, kind, spec, roles)
MACHINES = [
    ("mach-hypervisor", "Sample hypervisor", "hw-laptop", "hypervisor.lab.test",
     "192.0.2.10", "ExampleOS 1.0", "rig", "8 vCPU / 16 GiB / 512 GiB", ["virtualization"]),
    ("mach-app-vm", "Sample application VM", "mach-hypervisor", "app.lab.test",
     "192.0.2.11", "Ubuntu 22.04", "rig", "4 vCPU / 8 GiB / 100 GiB", ["deployment"]),
    ("mach-db-vm", "Sample database VM", "mach-hypervisor", "db.lab.test",
     "192.0.2.12", "Ubuntu 22.04", "rig", "4 vCPU / 16 GiB / 200 GiB", ["database"]),
    ("mach-home-server", "Sample home server", "hw-tower", "homeserver.lab.test",
     "198.51.100.20", "Ubuntu 24.04", "rig", "4 vCPU / 8 GiB / 50 GiB", ["storage"]),
    ("mach-gateway", "Sample gateway router", "hw-board", "gateway.lab.test",
     "198.51.100.1", "OpenWrt 23.05", "shared", None, ["network"]),
    ("mach-remote", "Sample remote VPS", None, "vps.lab.test",
     "203.0.113.30", "Debian 12", "rig", "2 vCPU / 4 GiB / 40 GiB", ["public-entry"]),
    ("mach-template", "Sample VM template", "mach-hypervisor", "template.lab.test",
     "192.0.2.100", "Ubuntu 22.04", "virtual", "2 vCPU / 4 GiB / 20 GiB", None),
    ("mach-retired", "Sample retired host", "hw-tower", "retired.lab.test",
     "198.51.100.99", "Debian 11", "rig", "2 vCPU / 4 GiB / 40 GiB", None),
]
for label, name, parent, hostname, ip, os, kind, spec, roles in MACHINES:
    lifecycle = "retired" if label == "mach-retired" else ("template" if label == "mach-template" else "active")
    link(label, "knowsAbout", ("Machine", name, lifecycle))
    nodes[-1]["machine"] = {
        "hostname": hostname,
        "ips": [ip],
        "os": os,
        "status": lifecycle,
        "machineKind": kind,
        "accessMethod": f"ssh ops@{ip}",
        "spec": spec,
        "roles": roles,
        "responsibilityBoundary": "Synthetic host used only for development fixtures",
        "allowedOperations": ["read logs", "restart services"],
        "forbiddenOperations": ["wipe storage"],
        "owned": ["synthetic owner"],
        "resources": [{"kind": "disk", "name": "root", "scope": "system"}],
    }
    if parent:
        relate(parent, "hosts", label)

# --- 服务（20 个）-------------------------------------------------------
# (label, name, host, product, ports, domains, deploy_path, notes)
SERVICES = [
    ("svc-web", "sample-web", "mach-app-vm", "nginx", [80, 443],
     ["web.lab.test"], "/opt/sample/web/", ["reverse proxy for lab services"]),
    ("svc-api", "sample-api", "mach-app-vm", "app-server", [8080],
     ["api.lab.test"], "/opt/sample/api/", ["stateless", "restart policy: unless-stopped"]),
    ("svc-cache", "sample-cache", "mach-app-vm", "redis", [6379],
     None, "/opt/sample/cache/", ["memory limit 512MiB"]),
    ("svc-db", "sample-db", "mach-db-vm", "postgres", [5432],
     None, "/opt/sample/db/", ["data volume on separate disk"]),
    ("svc-db-backup", "sample-db-backup", "mach-db-vm", "pg-dump", None,
     None, "/opt/sample/db/", ["nightly dump at 03:00", "retention 14 days"]),
    ("svc-registry", "sample-registry", "mach-home-server", "artifact-registry", [8081],
     ["registry.lab.test"], "/opt/sample/registry/", ["hosts internal build artifacts"]),
    ("svc-git", "sample-git", "mach-home-server", "git-server", [3000],
     ["git.lab.test"], "/opt/sample/git/", ["private repositories"]),
    ("svc-dashboard", "sample-dashboard", "mach-home-server", "dashboard", [80],
     ["dash.lab.test"], "/opt/sample/dashboard/", ["start page for lab links"]),
    ("svc-metrics", "sample-metrics", "mach-home-server", "metrics-agent", [9090],
     None, "/opt/sample/metrics/", ["scrapes all hosts every 30s"]),
    ("svc-dns", "sample-dns", "mach-gateway", "dns-resolver", [53],
     None, "/etc/sample/dns/", ["authoritative for lab.test"]),
    ("svc-vpn", "sample-vpn", "mach-gateway", "vpn-gateway", [51820],
     None, "/etc/sample/vpn/", ["wireguard", "peers managed by script"]),
    ("svc-wiki", "sample-wiki", "mach-remote", "wiki", [443],
     ["wiki.lab.test"], "/srv/sample/wiki/", ["public documentation site"]),
    ("svc-sync", "sample-sync", "mach-app-vm", "file-sync", [8384, 22000],
     None, "/opt/sample/sync/", ["syncs documents between two hosts"]),
    ("svc-jump", "sample-jump", "mach-remote", "bastion", [22],
     None, "/etc/sample/jump/", ["single entry point for remote access"]),
    ("svc-scheduler", "sample-scheduler", "mach-app-vm", "job-scheduler", [8082],
     None, "/opt/sample/scheduler/", ["runs cron-like jobs"]),
    ("svc-log", "sample-log", "mach-db-vm", "log-store", [9200],
     None, "/opt/sample/log/", ["rotates at 1GiB"]),
    ("svc-object", "sample-object", "mach-home-server", "object-store", [9000],
     ["files.lab.test"], "/opt/sample/object/", ["stores build artifacts"]),
    ("svc-proxy", "sample-proxy", "mach-remote", "http-proxy", [3128],
     None, "/etc/sample/proxy/", ["egress proxy for lab hosts"]),
    ("svc-template-app", "sample-template-app", "mach-template", "app-server", [8080],
     None, "/opt/sample/template/", ["baked into the VM template"]),
    ("svc-retired-app", "sample-retired-app", "mach-retired", "app-server", [8080],
     None, "/opt/sample/retired/", ["decommissioned, kept for history"]),
]
for label, name, host, product, ports, domains, path, notes in SERVICES:
    lifecycle = "stopped" if label == "svc-retired-app" else "active"
    link(label, "knowsAbout", ("SoftwareInstance", name, lifecycle))
    detail = {
        "productName": product,
        "deploymentPath": path,
        "deploymentState": "deployed",
        "operationalState": "active" if lifecycle == "active" else "stopped",
        "notes": notes,
        "configurationReferences": [f"synthetic://config/{label}"],
    }
    if ports:
        detail["ports"] = ports
        detail["endpoints"] = [{"kind": "tcp", "address": "127.0.0.1", "port": ports[0]}]
    if domains:
        detail["domains"] = domains
    if product not in ("app-server", "pg-dump"):
        detail["runtimeIdentity"] = f"sample-{product}"
    nodes[-1]["softwareInstance"] = detail
    relate(host, "hosts", label)

# --- 凭据（6 个）--------------------------------------------------------
CREDENTIALS = [
    ("cred-web", "web-admin", "mach-app-vm", ["svc-web"], "console login", "service account"),
    ("cred-api", "api-token", "mach-app-vm", ["svc-api"], "API authentication", "rotating token"),
    ("cred-db", "db-owner", "mach-db-vm", ["svc-db"], "database owner", "service account"),
    ("cred-git", "git-admin", "mach-home-server", ["svc-git"], "repository administration", "personal account"),
    ("cred-registry", "registry-publisher", "mach-home-server", ["svc-registry"], "artifact publishing", "service account"),
    ("cred-jump", "jump-operator", "mach-remote", ["svc-jump"], "remote access", "shared account"),
]
for label, name, host, services, purpose, kind in CREDENTIALS:
    link(label, "knowsAbout", ("AccountCredential", name, "active"))
    nodes[-1]["accountCredential"] = {
        "accessReference": f"synthetic://credentials/{name}",
        "account": name,
        "credentialType": kind,
        "purpose": purpose,
        "scope": "lab.test",
        "status": "active",
        "lastVerified": "2026-01-15T00:00:00Z",
        "rotationAt": "2026-07-15T00:00:00Z",
    }
    for svc in services:
        relate(svc, "hasCredential", label)

# --- 文档 / 手册 / 项目 / 网络 ------------------------------------------
link("doc-runbook", "knowsAbout", ("Document", "Sample operations handbook", "active"))
nodes[-1]["document"] = {
    "role": "how the sample lab is laid out",
    "effective": True,
    "authoritativeReference": "synthetic://docs/handbook",
    "lastVerified": "2026-01-15T00:00:00Z",
}

link("rb-access", "knowsAbout", ("Runbook", "Sample access runbook", "active"))
nodes[-1]["runbook"] = {
    "approvalRequired": True,
    "executableStatus": "verified",
    "preconditions": ["VPN connected", "jump host reachable"],
    "verificationMethod": "run the check command and compare output",
    "stepsReference": "synthetic://runbooks/access",
    "rollbackReference": "synthetic://runbooks/access-rollback",
    "trigger": "need to reach a lab host",
    "scope": "lab access",
}
relate("rb-access", "appliesTo", "svc-jump")
relate("doc-runbook", "documents", "svc-web")

link("proj-sample", "owns", ("SoftwareProject", "sample-service", "active"))
nodes[-1]["softwareProject"] = {
    "form": "service",
    "languages": ["Go", "TypeScript"],
    "maintenanceState": "active",
    "purpose": "demonstrates a service deployed across the lab",
    "remoteRepository": "synthetic://repos/sample-service",
    "upstream": "synthetic://upstream/sample-service",
}
relate("svc-api", "implementedFrom", "proj-sample")
relate("svc-web", "implementedFrom", "proj-sample")

link("net-lab", "knowsAbout", ("Network", "Sample lab network", "active"))
nodes[-1]["network"] = {
    "kind": "lan",
    "cidrs": ["192.0.2.0/24", "198.51.100.0/24"],
    "status": "active",
    "gatewayReference": "synthetic://access/gateway",
    "vpnReference": "synthetic://access/vpn",
    "accessLocations": ["home", "office"],
}
relate("net-lab", "providesAccessTo", "mach-gateway")

# --- 审查区草稿 ---------------------------------------------------------
link("prov-unknown", "knowsAbout", ("ProvisionalObject", "Unclassified sample host", None))
nodes[-1].update({
    "certainty": "provisional",
    "candidateType": "Machine",
    "reviewStatus": "pending",
    "reviewQuestions": ["Is this a physical machine or a VM?", "Which hardware does it belong to?"],
})

batch = {
    "source": "synthetic-dev-seed",
    "parserVersion": "seed-v1",
    "nodes": nodes,
    "relations": relations,
    "assertions": [],
}

print(json.dumps(batch, indent=2, ensure_ascii=False))
