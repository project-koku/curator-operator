#! /bin/bash

TMP_DIR=$(mktemp -d)

git clone https://github.com/coreos/kube-prometheus.git "${TMP_DIR}"

oc apply -f "${TMP_DIR}"/manifests/setup/
until kubectl get servicemonitors --all-namespaces ; do date; sleep 1; echo ""; done
oc apply -f "${TMP_DIR}"/manifests/

rm -rf "${TMP_DIR}"

echo "Grabbing the ClusterIP of the prometheus-k8s Service"
CLUSTER_IP="$(oc -n monitoring get service prometheus-k8s -o jsonpath='{.spec.clusterIP}')"
echo "Cluster IP of the prometheus.k8s.svc: ${CLUSTER_IP}:9090"
