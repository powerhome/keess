#!/usr/bin/env bash

# We should convert this to the e2e-framework (https://github.com/kubernetes-sigs/e2e-framework)

set -x

use_kind=true
keess_docker_image='leonardogodoyphrg/keess:1.2.0'
kubeconfig_reloader_docker_image='leonardogodoyphrg/keess-kubeconfig-reloader:1.0.0'
cleaningup=false

clusters=(
    cluster-a
    cluster-b
    cluster-c
    cluster-d
)
mapfile -t kind_clusters < <(echo "${clusters[@]/#/kind-}" | tr ' ' '\n')

function cleanup() {
    if [ "$cleaningup" = true ]; then
        return
    fi
    cleaningup=true
    if [[ "$use_kind" == true ]]; then
        for cluster in "${clusters[@]}"; do
            kind delete cluster --name "$cluster"
        done
    else
        gsed 's|<cluster>|wc-beta-gm|g' ./base.yaml | kubectl --context wc-beta-gm delete -f -
        gsed 's|<cluster>|wc-prod-hq|g' ./base.yaml | kubectl --context wc-prod-hq delete -f -
    fi
    exit 0
}
trap cleanup EXIT ERR SIGINT SIGTERM


if [[ "$use_kind" == true ]]; then
    docker buildx build -t $keess_docker_image .
    docker buildx build -t $kubeconfig_reloader_docker_image -f Dockerfile.kubeconfig-reloader .
else
    for platform in linux/arm64 linux/amd64; do
        docker buildx build --platform $platform -t $keess_docker_image .
        docker buildx build --platform $platform -t $kubeconfig_reloader_docker_image -f Dockerfile.kubeconfig-reloader .
    done
    docker push $keess_docker_image
    docker push $kubeconfig_reloader_docker_image
fi

export KUBECONFIG="$HOME/.kube/kind"

if [[ "$use_kind" == true ]]; then
    for cluster in "${clusters[@]}"; do
        kind create cluster --config "$HOME/projetos/local/testes-locais/kind/kind-config.yaml" --name "$cluster"
        kind load docker-image "$keess_docker_image" "$kubeconfig_reloader_docker_image" --name "$cluster"
        for namespace in "${clusters[@]/#/from-}"; do
            kubectl create namespace "$namespace" --context "kind-$cluster"
        done
        gsed "s|<cluster>|kind-$cluster|g" ./base.yaml | kubectl --context "kind-$cluster" apply -f -
        echo "sleeping..."
        gsleep 15
    done
    # kubectl view-secret -n keess keess-token token --context kind-cluster-a | sudo gtee /var/run/secrets/kubernetes.io/serviceaccount/token
    # kubectl view-secret -n keess keess-token ca.crt --context kind-cluster-a | sudo gtee /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    for cluster in "${clusters[@]}"; do
        mapfile -t filtered_clusters < <(echo "${kind_clusters[@]/kind-$cluster}" | tr ' ' '\n')
        kubectl apply --context "kind-$cluster" -f - <<EOF
---
apiVersion: v1
kind: ConfigMap
metadata:
    name: test-from-$cluster
    namespace: from-$cluster
    labels:
        keess.powerhrg.com/sync: cluster
    annotations:
        keess.powerhrg.com/clusters: "$(echo ${filtered_clusters[@]} | tr ' ' ',')"
data:
    example.key: example-value
    random-value: "$(shuf -i 1-100000 -n 1)"
---
apiVersion: v1
kind: Secret
metadata:
    name: test-from-$cluster
    namespace: from-$cluster
    labels:
        keess.powerhrg.com/sync: cluster
    annotations:
        keess.powerhrg.com/clusters: "$(echo ${filtered_clusters[@]} | tr ' ' ',')"
type: Opaque
data:
    username: dXNlcm5hbWU= # base64 encoded value of "username"
    password: cGFzc3dvcmQ= # base64 encoded value of "password"
    random-value: "$(shuf -i 1-100000 -n 1 | base64)"
---
apiVersion: v1
kind: Namespace
metadata:
    name: should-be-empty
---
apiVersion: v1
kind: Namespace
metadata:
    name: new-default
    labels:
        keess.powerhrg.com/sync: "true"
---
apiVersion: v1
kind: ConfigMap
metadata:
    name: test-from-ns-default
    namespace: default
    labels:
        keess.powerhrg.com/sync: namespace
    annotations:
        keess.powerhrg.com/namespace-label: keess.powerhrg.com/sync="true"
data:
    example.key: example-value
    random-value: "$(shuf -i 1-100000 -n 1)"
---
apiVersion: v1
kind: Secret
metadata:
    name: test-from-ns-default
    namespace: default
    labels:
        keess.powerhrg.com/sync: namespace
    annotations:
        keess.powerhrg.com/namespace-label: keess.powerhrg.com/sync="true"
type: Opaque
data:
    username: dXNlcm5hbWU= # base64 encoded value of "username"
    password: cGFzc3dvcmQ= # base64 encoded value of "password"
    random-value: "$(shuf -i 1-100000 -n 1 | base64)"
EOF
    done
    while true; do
        for kind_cluster in "${kind_clusters[@]}"; do
            for cluster in "${clusters[@]}"; do
                kubectl get cm,secret -n "from-$cluster" --context "$kind_cluster"
            done
        done
        gsleep 5
    done
else
    gsed 's|<cluster>|wc-beta-gm|g' ./base.yaml | kubectl --context wc-beta-gm apply -f -
    gsed 's|<cluster>|wc-prod-hq|g' ./base.yaml | kubectl --context wc-prod-hq apply -f -
fi

echo 'sleeping until interrupted...'
gsleep infinity
