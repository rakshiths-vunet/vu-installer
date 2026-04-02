# Ansible Installer API Usage Guide

This document provides a reference for interacting with the Ansible Installer service using `curl`.

## Base Configuration

When accessing the service through Kong/Traefik, you must provide the correct `Host` header.

- **Base URL**: `http://10.1.92.30:8086` (as per your setup)
- **Host Header**: `portal.vunetlocalserver.com`
- **Prefix**: `/ansible` (based on your example usage, requests are routed via this prefix)

## Common Flags
- `-k`: Allow insecure server connections (useful for self-signed certificates).
- `-H`: Add a custom header (used for `Host` and `Content-Type`).
- `-X`: Specify the request method (GET, POST, etc.).
- `-d`: Send data/body (for POST requests).
- `-v`: Verbose mode (optional, for debugging).

---

## Endpoints

### 1. Health Check
Check if the service is running and healthy.

**Command:**
```bash
curl -k -H "Host: portal.vunetlocalserver.com" \
"http://10.1.92.30:8086/ansible/health"
```

**Expected Outcome:**
```json
{
  "status": "healthy",
  "developer": "Sid & Team"
}
```

---

### 2. Install Node
Trigger a new installation on a target node.

**Command:**
```bash
curl -k -H "Host: portal.vunetlocalserver.com" \
-H "Content-Type: application/json" \
-X POST \
-d '{
    "name": "node1",
    "ip": "192.168.1.100",
    "user": "vunet",
    "version": "2.16"
}' \
"http://10.1.92.30:8086/ansible/install"
```

> **Note:** `user` defaults to "vunet" and `key` defaults to "~/.ssh/id_rsa" if not specified.

**Expected Outcome (Success):**
```json
{
  "status": "accepted",
  "message": "Installation started"
}
```

**Expected Outcome (Conflict/Busy):**
```json
Node is busy
```

---

### 3. Get Node Status
Retrieve the detailed status of a specific node's installation.

**Command:**
```bash
curl -k -H "Host: portal.vunetlocalserver.com" \
"http://10.1.92.30:8086/ansible/status?name=node1"
```

**Expected Outcome (In Progress):**
```json
{
  "node_name": "node1",
  "ip": "192.168.1.100",
  "status": "RUNNING",
  "step": "Starting",
  "start_time": "2026-01-19T08:59:14Z",
  "locked": true,
  "tasks": [],
  "error_msg": ""
}
```

**Expected Outcome (Failed):**
```json
{
  "node_name": "node1",
  "ip": "192.168.1.100",
  "status": "FAILED",
  "step": "Configuring",
  "start_time": "2026-01-19T08:59:14Z",
  "locked": false,
  "tasks": [...],
  "error_msg": "exit status 2"
}
```

---

### 4. List All Nodes
Get a summary list of all nodes known to the installer.

**Command:**
```bash
curl -k -H "Host: portal.vunetlocalserver.com" \
"http://10.1.92.30:8086/ansible/nodes"
```

**Expected Outcome:**
```json
[
  {
    "node_name": "node1",
    "ip": "192.168.1.100",
    "version": "2.16",
    "status": "SUCCESS",
    "has_vsmaps": true,
    "start_time": "2026-01-19T09:00:00Z",
    "step": "Completed"
  }
]
```

---

### 5. Retry Installation
Retry an installation for a node that is in a `FAILED` state.

**Command:**
```bash
curl -k -H "Host: portal.vunetlocalserver.com" \
-H "Content-Type: application/json" \
-X POST \
-d '{"name": "node1"}' \
"http://10.1.92.30:8086/ansible/retry"
```

**Expected Outcome:**
```json
{
  "status": "accepted",
  "message": "Retry started"
}
```

---

### 6. Install 3-Node Cluster (Version 3node3.0)
Trigger a 3-node cluster installation. Prereqs will be run on all nodes, but the main installer runs on the primary node (specified by `ip1`).

**Command:**
```bash
curl -k -H "Host: portal.vunetlocalserver.com" \
-H "Content-Type: application/json" \
-X POST \
-d '{
    "name": "cluster-deployment",
    "version": "3node3.0",
    "ip": "172.23.0.8",
    "ip1": "172.23.0.8",
    "ip2": "172.23.0.9",
    "ip3": "172.23.0.11",
    "vmname1": "karthik-1-ubuntu",
    "vmname2": "karthik-2-ubuntu",
    "vmname3": "karthik-3-ubuntu",
    "user": "vunet"
}' \
"http://10.1.92.30:8086/ansible/install"
```

**Expected Outcome:** Same as single node installation.

---
*Maintained by **Sid & Team***
