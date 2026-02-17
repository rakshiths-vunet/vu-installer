# vu-installer (VuSmartMaps One-Touch Installer)

**vu-installer** is a robust, Go-based orchestration service designed to automate the installation and configuration of **VuSmartMaps** nodes. It acts as a bridge between a user-friendly REST API/Frontend and powerful Ansible automation, managing the lifecycle of installations across multiple nodes.

## 🚀 Overview

The project enables a "One-Touch" installation experience. Instead of manually running Ansible playbooks and tracking logs, users (or a frontend portal) interact with this service to:
1.  **Trigger Installations**: Provision new nodes with specific configurations.
2.  **Monitor Progress**: Track real-time status, completed steps, and logs.
3.  **Handle Failures**: Smart retry mechanism that resumes execution *exactly* from the point of failure.
4.  **Manage State**: Persist installation state across service restarts using SQLite.

## ✨ Key Features

-   **RESTful API**: Clean JSON API for integration with portals (like `portal.vunetlocalserver.com`).
-   **State Management**: Uses SQLite (`installer.db`) to track the status, Start Time, IP, User, and Progress of every node.
-   **Smart Retry**: If an installation fails (e.g., network timeout during package download), the `retry` endpoint identifies the failed Ansible task and resumes execution from that specific task, skipping successful prerequisites.
-   **Ansible Integration**: Dynamically generates inventories and executes playbooks (`site.yml`) with custom variables.
-   **Log Monitoring**: Streams and monitors detailed Ansible logs (`ansible_<node>.log`) and installation verification logs.
-   **Concurrency**: Handles multiple node installations concurrently via Go routines.

## 🏗️ Architecture

```mermaid
graph TD
    User[User / Portal] -->|Provisions Node| API[Go HTTP Server]
    API -->|Persists State| DB[(SQLite DB)]
    API -->|Generates| Inv[Inventory.ini]
    API -->|Executes| Ansible[Ansible Playbook]
    Ansible -->|Configures| Node[Target Node (VM)]
    Ansible -->|Writes| Logs[Log Files]
    API -->|Monitors| Logs
```

## 🛠️ Directory Structure

```text
vu-installer/
├── cmd/
│   └── main.go              # Entry point: HTTP Server & API Handlers
├── internal/
│   ├── ansible/             # Ansible runner logic & log monitoring
│   └── state/               # DB logic for install states (SQLite)
├── playbooks/
│   ├── roles/               # Ansible roles (vusmartmaps, etc.)
│   └── site.yml             # Main playbook entry point
├── configs/                 # Sizing and validation configs
├── configs/                 # Sizing and validation configs
├── installer.db             # SQLite database (auto-created)
├── API_USAGE.md             # Detailed API documentation
└── README.md                # This file
```

## 📋 Prerequisites

-   **Go**: Version 1.16+
-   **Ansible**: Installed on the host machine (`apt install ansible`).
-   **SQLite3**: For state management.
-   **SSH Access**: The host running this service must have SSH access to target nodes (default key: `~/.ssh/id_rsa`).

## ⚙️ Setup & Installation

1.  **Clone the repository**:
    ```bash
    git clone <repo_url>
    cd vu-installer
    ```

2.  **Build the binary**:
    ```bash
    go build -o installer cmd/main.go
    ```

3.  **Run the service**:
    ```bash
    # Runs on default port 8081
    ./installer
    ```
    *Optionally specify a port:*
    ```bash
    PORT=8086 ./installer
    ```

## 🔌 API Reference

See [API_USAGE.md](./API_USAGE.md) for detailed `curl` examples and request/response schemas.

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/ansible/health` | `GET` | Service health check |
| `/ansible/install` | `POST` | Start a new installation |
| `/ansible/status` | `GET` | Get status of a specific node |
| `/ansible/nodes` | `GET` | List all nodes and their statuses |
| `/ansible/retry` | `POST` | Retry a failed installation from failure point |

## 🧩 Usage Example

**Start an installation via Curl:**

```bash
curl -k -H "Host: portal.vunetlocalserver.com" \
-H "Content-Type: application/json" \
-X POST \
-d '{
    "name": "Node-Alpha",
    "ip": "192.168.1.50",
    "version": "3.0"
}' \
http://localhost:8086/ansible/install
```

The service will:
1.  Lock the node to prevent duplicate runs.
2.  Generate an inventory entry for `192.168.1.50`.
3.  Execute the playbook.
4.  Update the internal DB with step-by-step progress.

## 🤝 Contribution

1.  Tasks are defined in `playbooks/roles/vusmartmaps/tasks/`.
2.  Go application logic is in `cmd/` and `internal/`.
3.  Ensure database schema changes are reflected in `internal/state/manager.go`.
