#!/bin/bash

#############################################################################
# Create Spoke EKS Clusters
#############################################################################
#
# DESCRIPTION:
#   This script creates the spoke EKS clusters in different regions. It:
#   1. Configures spoke cluster accounts in ArgoCD for ACK controller
#   2. Updates cluster definitions with management account ID and Git URLs
#   3. Enables and configures the fleet spoke clusters
#   4. Syncs the clusters application in ArgoCD
#   5. Creates the EKS clusters using KRO
#
# USAGE:
#   ./3-create-spoke-clusters.sh
#
# PREREQUISITES:
#   - Management and spoke accounts must be bootstrapped (run 2-bootstrap-accounts.sh first)
#   - ArgoCD must be configured and accessible
#   - Environment variables must be set:
#     - MGMT_ACCOUNT_ID: AWS Management account ID
#     - WORKSPACE_PATH: Path to the workspace directory
#     - WORKING_REPO: Name of the working repository
#     - GITLAB_URL: URL of the GitLab instance
#     - GIT_USERNAME: Git username for authentication
#
# SEQUENCE:
#   This is the fourth script (3) in the setup sequence.
#   Run after 2-bootstrap-accounts.sh and before 4-deploy-argo-rollouts-demo.sh
#
#############################################################################

echo "Configure spoke cluster accounts in Argo CD application for ACK controller:"

sed -i 's/MANAGEMENT_ACCOUNT_ID/'"$MGMT_ACCOUNT_ID"'/g' "$WORKSPACE_PATH/$WORKING_REPO/addons/tenants/tenant1/default/addons/multi-acct/values.yaml"

echo "activate the account numbers"
sed -i 's/# \(cluster-test: "[0-9]*"\)/\1/g; s/# \(cluster-pre-prod: "[0-9]*"\)/\1/g; s/# \(cluster-prod-eu: "[0-9]*"\)/\1/g; s/# \(cluster-prod-us: "[0-9]*"\)/\1/g' /home/ec2-user/environment/eks-cluster-mgmt/addons/tenants/tenant1/default/addons/multi-acct/values.yaml

/usr/lib/code-server/bin/code-server $WORKSPACE_PATH/$WORKING_REPO/addons/tenants/tenant1/default/addons/multi-acct/values.yaml

cd $WORKSPACE_PATH/$WORKING_REPO/
git status
git add .
git commit -m "add namespaces and resources for clusters"
git push

echo "Sync the cluster-workloads application"
argocd app sync multi-acct-hub-cluster

echo "Wait for the rollouts-demo-kargo application to be synced and healthy"
argocd app wait multi-acct-hub-cluster --health --sync

echo "Update cluster definitions with Management account ID:"
sed -i 's/MANAGEMENT_ACCOUNT_ID/'"$MGMT_ACCOUNT_ID"'/g' "$WORKSPACE_PATH/$WORKING_REPO/fleet/kro-values/tenants/tenant1/kro-clusters/values.yaml"
sed -i 's|GITLAB_URL|'"$GITLAB_URL"'|g' "$WORKSPACE_PATH/$WORKING_REPO/fleet/kro-values/tenants/tenant1/kro-clusters/values.yaml"
sed -i 's/GIT_USERNAME/'"$GIT_USERNAME"'/g' "$WORKSPACE_PATH/$WORKING_REPO/fleet/kro-values/tenants/tenant1/kro-clusters/values.yaml"
sed -i 's/WORKING_REPO/'"$WORKING_REPO"'/g' "$WORKSPACE_PATH/$WORKING_REPO/fleet/kro-values/tenants/tenant1/kro-clusters/values.yaml"

echo "update fleet spoke clusters"
sed -i '
# First uncomment the section headers
s/^  # cluster-test:/  cluster-test:/g
s/^  # cluster-pre-prod:/  cluster-pre-prod:/g
s/^  # cluster-prod-us:/  cluster-prod-us:/g
s/^  # cluster-prod-eu:/  cluster-prod-eu:/g

# Then uncomment the content under each section, but stop before workload-cluster1
/^  cluster-test:/,/^  cluster-pre-prod:/ {
  s/^  #/  /g
}
/^  cluster-pre-prod:/,/^  cluster-prod-us:/ {
  s/^  #/  /g
}
/^  cluster-prod-us:/,/^  cluster-prod-eu:/ {
  s/^  #/  /g
}
/^  cluster-prod-eu:/,/^  # workload-cluster1:/ {
  /^  # workload-cluster1:/!s/^  #/  /g
}' /home/ec2-user/environment/eks-cluster-mgmt/fleet/kro-values/tenants/tenant1/kro-clusters/values.yaml

/usr/lib/code-server/bin/code-server $WORKSPACE_PATH/$WORKING_REPO/fleet/kro-values/tenants/tenant1/kro-clusters/values.yaml

cd $WORKSPACE_PATH/$WORKING_REPO/
git status
git add .
git commit -m "add clusters definitions"
git push

argocd app sync clusters

kubectl get EksClusterwithvpcs -A

echo "Spoke EKS clusters creation initiated."
echo "Next step: Run 4-deploy-argo-rollouts-demo.sh to deploy the Argo Rollouts demo application."
