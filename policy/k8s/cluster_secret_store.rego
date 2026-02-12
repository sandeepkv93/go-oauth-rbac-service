package k8s.clustersecretstore

is_cluster_secret_store {
  input.apiVersion == "external-secrets.io/v1beta1"
  input.kind == "ClusterSecretStore"
}

deny[msg] {
  is_cluster_secret_store
  object.get(input.metadata, "name", "") != "secure-observable-secret-store"
  msg := "ClusterSecretStore name must be secure-observable-secret-store"
}

deny[msg] {
  is_cluster_secret_store
  not input.spec.provider
  msg := "ClusterSecretStore must define spec.provider"
}

deny[msg] {
  is_cluster_secret_store
  provider := input.spec.provider
  count(provider) == 0
  msg := "ClusterSecretStore provider block must not be empty"
}

deny[msg] {
  is_cluster_secret_store
  provider := input.spec.provider
  count(provider) > 1
  msg := "ClusterSecretStore must define exactly one provider"
}

is_aws_provider {
  is_cluster_secret_store
  input.spec.provider.aws
}

has_aws_jwt_auth {
  is_aws_provider
  sa := object.get(input.spec.provider.aws.auth.jwt, "serviceAccountRef", {})
  object.get(sa, "name", "") != ""
  object.get(sa, "namespace", "") == "external-secrets"
}

has_aws_static_auth {
  is_aws_provider
  access := object.get(input.spec.provider.aws.auth.secretRef, "accessKeyIDSecretRef", {})
  secret := object.get(input.spec.provider.aws.auth.secretRef, "secretAccessKeySecretRef", {})
  object.get(access, "name", "") != ""
  object.get(access, "key", "") != ""
  object.get(access, "namespace", "") == "external-secrets"
  object.get(secret, "name", "") != ""
  object.get(secret, "key", "") != ""
  object.get(secret, "namespace", "") == "external-secrets"
}

deny[msg] {
  is_aws_provider
  not has_aws_jwt_auth
  not has_aws_static_auth
  msg := "AWS ClusterSecretStore must configure auth via jwt.serviceAccountRef or secretRef credentials in namespace external-secrets"
}

is_vault_provider {
  is_cluster_secret_store
  input.spec.provider.vault
}

has_vault_token_auth {
  is_vault_provider
  ref := object.get(input.spec.provider.vault.auth, "tokenSecretRef", {})
  object.get(ref, "name", "") != ""
  object.get(ref, "key", "") != ""
  object.get(ref, "namespace", "") == "external-secrets"
}

has_vault_kubernetes_auth {
  is_vault_provider
  k8s_auth := object.get(input.spec.provider.vault.auth, "kubernetes", {})
  object.get(k8s_auth, "mountPath", "") != ""
  object.get(k8s_auth, "role", "") != ""
  sa := object.get(k8s_auth, "serviceAccountRef", {})
  object.get(sa, "name", "") != ""
  object.get(sa, "namespace", "") == "external-secrets"
}

deny[msg] {
  is_vault_provider
  server := object.get(input.spec.provider.vault, "server", "")
  server == ""
  msg := "Vault ClusterSecretStore must define provider.vault.server"
}

deny[msg] {
  is_vault_provider
  server := object.get(input.spec.provider.vault, "server", "")
  not startswith(server, "https://")
  msg := "Vault ClusterSecretStore server must use https://"
}

deny[msg] {
  is_vault_provider
  not has_vault_token_auth
  not has_vault_kubernetes_auth
  msg := "Vault ClusterSecretStore must configure auth via tokenSecretRef or kubernetes service account auth in namespace external-secrets"
}
