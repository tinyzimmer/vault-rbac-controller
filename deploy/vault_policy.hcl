# Create and manage Kubernetes auth roles
# Replace the path with where you configured the Kubernetes auth method
path "auth/kubernetes/role/*" {
    capabilities = ["read", "list", "create", "update", "delete"]
}

# Create and manage ACL policies
path "sys/policies/acl/*" {
  capabilities = ["create", "read", "update", "delete", "list", "sudo"]
}