#!/usr/bin/env bash

# LOCAL_CLUSTER_NAME must be set

KUBE_PUBLIC_CLUSTER_INFO=$(kubectl get cm -n kube-public cluster-info -o yaml)
KUBERNETES_SERVER_URL="$(echo "$KUBE_PUBLIC_CLUSTER_INFO" | yq '.data.kubeconfig | from_yaml | .clusters[0].cluster.server')"
KUBERNETES_CLUSTER_CA=$(echo "$KUBE_PUBLIC_CLUSTER_INFO" | yq '.data.kubeconfig | from_yaml | .clusters[0].cluster.certificate-authority-data')

SERVICE_ACCOUNT_DIR="/var/run/secrets/kubernetes.io/serviceaccount"
KUBERNETES_NAMESPACE=$(cat "$SERVICE_ACCOUNT_DIR"/namespace)
KUBERNETES_USER_TOKEN=$(cat "$SERVICE_ACCOUNT_DIR"/token)
mkdir -p "$HOME"/.kube/configs

cat > "$HOME/.kube/configs/$LOCAL_CLUSTER_NAME" <<EOF
apiVersion: v1
kind: Config
preferences: {}
current-context: null
clusters:
- name: $LOCAL_CLUSTER_NAME
  cluster:
    server: $KUBERNETES_SERVER_URL
    certificate-authority-data: $KUBERNETES_CLUSTER_CA
users:
- name: $LOCAL_CLUSTER_NAME-keessServiceAccount
  user:
    token: $KUBERNETES_USER_TOKEN
contexts:
- context:
    cluster: $LOCAL_CLUSTER_NAME
    user: $LOCAL_CLUSTER_NAME-keessServiceAccount
    namespace: $KUBERNETES_NAMESPACE
  name: $LOCAL_CLUSTER_NAME
EOF

set -x

# Sanitize $S3_BUCKET_PATH to remove leading and trailing slashes
S3_BUCKET_PATH=$(echo "$S3_BUCKET_PATH" | sed 's|^/||;s|/$||')

s5cmd --endpoint-url "$S3_ENDPOINT" \
      --credentials-file "$CREDENTIALS_FILE" \
      sync "$HOME/.kube/configs/$LOCAL_CLUSTER_NAME" "s3://$S3_BUCKET/$S3_BUCKET_PATH/$LOCAL_CLUSTER_NAME"

while true; do
    s5cmd --endpoint-url "$S3_ENDPOINT" \
          --credentials-file "$CREDENTIALS_FILE" \
          sync --delete --exclude "$LOCAL_CLUSTER_NAME" "s3://$S3_BUCKET/$S3_BUCKET_PATH/*" /root/.kube/configs

    KUBECONFIG=$(find "$HOME"/.kube/configs -type f | sort | tr '\n' ':')
    export KUBECONFIG
    kubectl config view --flatten > /root/.kube/config
    unset KUBECONFIG
    sleep 60
done
