# MCSP Customer Onboarding Operator

A Go operator built using Operator SDK that automates customer onboarding on Red Hat OpenShift. The operator acts as an orchestrator — it coordinates existing operators like RHACM, Cert Manager and External Secrets to provision complete isolated customer environments automatically by just applying one MCPSCustomer CR to the cluster.

---

## What The Operator Does

When a MCPSCustomer CR is applied to the cluster the operator:

- Creates an RHACM Policy and PlacementBinding → RHACM automatically provisions the namespace with resource quotas and RBAC permissions
- Creates a Certificate → Cert Manager automatically provisions a TLS certificate for the customer URL
- Creates an ExternalSecret → External Secrets automatically fetches and injects customer credentials into the namespace
- Creates a Deployment, Service and Route → customer application is live at a unique HTTPS URL
- Updates the MCPSCustomer CR status with the live URL and deployment details

---

## Architecture
```
Apply MCPSCustomer CR
        ↓
Go Operator detects new CR
        ↓
Orchestrates existing operators:
├── RHACM → namespace + quotas + RBAC
├── Cert Manager → TLS certificate
└── External Secrets → credentials
        ↓
Creates directly:
├── Deployment → running pods
├── Service → internal networking
└── Route → public HTTPS URL
        ↓
Customer environment live!
```

---

## Prerequisites

- Red Hat OpenShift 4.x
- RHACM (Red Hat Advanced Cluster Management)
- Cert Manager
- External Secrets Operator
- Go 1.21+
- Operator SDK v1.x

---

## Project Structure
```
mcsp-operator/
├── api/v1/
│   └── mcpscustomer_types.go       → MCPSCustomer CRD definition
├── internal/controller/
│   └── mcpscustomer_controller.go  → Reconcile logic
├── config/
│   ├── crd/                        → Generated CRD manifests
│   └── rbac/                       → RBAC permissions
└── cmd/
    └── main.go                     → Operator entry point
```

---

## What Gets Created Per Customer

| Resource | Created By | Purpose |
|---|---|---|
| Namespace | RHACM | Isolated environment |
| ResourceQuota | RHACM | Resource limits |
| RoleBinding | RHACM | Image pull permissions |
| Certificate | Cert Manager | TLS for HTTPS URL |
| ExternalSecret | External Secrets | Inject credentials |
| Deployment | Operator | Run the application |
| Service | Operator | Internal networking |
| Route | Operator | Public HTTPS URL |