#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export DOMAIN_NAME=$($SCRIPT_DIR/get_cloudfront.sh)
export NLB_DNS=$($SCRIPT_DIR/get_nlb.sh)

echo Deploying ArgoCD ...

cd ~/environment
git clone git@$NLB_DNS:user1/eks-cluster-mgmt.git
mkdir -p ~/environment/cluster-management/argocd/base
wget -O ~/environment/cluster-management/argocd/base/install.yaml https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

kubectl create namespace argocd
kubectl apply -n argocd -f ~/environment/cluster-management/argocd/base/install.yaml
kubectl patch svc argocd-server -n argocd --type=merge -p '{"spec":{"type":"LoadBalancer"}}'
sleep 30
export ARGOCD_URL=http://$(kubectl get svc argocd-server -n argocd -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
curl --head -X GET --retry 20 --retry-all-errors --retry-delay 15 \
  --connect-timeout 5 --max-time 10 -k $ARGOCD_URL
echo "ArgoCD url: $ARGOCD_URL"
echo "ArgoCD username: admin"
export ARGOCD_PWD=$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
echo "ArgoCD password: $ARGOCD_PWD"

export GITLAB_URL=https://$NLB_DNS/gitlab

envsubst << 'EOF' | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: creds-gitlab
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: repo-creds
stringData:
  url: $GITLAB_URL
  type: git
  password: $IDE_PASSWORD
  username: user1
EOF

cat <<EOF > ~/environment/cluster-management/argocd/base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: argocd
resources:
  - install.yaml
EOF

mkdir -p ~/environment/cluster-management/argocd/management
cat <<EOF > ~/environment/cluster-management/argocd/management/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: argocd
resources:
  - install.yaml
EOF

cat <<EOF > ~/environment/cluster-management/argocd/management/patch-cm.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
data:
  timeout.reconciliation: "15s"
EOF

git add .
git commit -m "Initial commit"
git push --set-upstream origin main

envsubst << 'EOF' | kubectl apply -f -
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  finalizers:
  - argoproj.io/resources-finalizer
  name: argocd-management
  namespace: argocd
spec:
  destination:
    server: https://kubernetes.default.svc
  project: default
  source:
    path: ./argocd/management
    repoURL: $GITLAB_URL/user1/eks-cluster-mgmt.git
  syncPolicy: {}
EOF
