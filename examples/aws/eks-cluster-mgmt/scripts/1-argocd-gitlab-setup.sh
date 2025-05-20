#!/bin/bash

#############################################################################
# ArgoCD and GitLab Setup Script
#############################################################################
#
# DESCRIPTION:
#   This script configures ArgoCD and GitLab for the EKS cluster management
#   environment. It:
#   1. Updates the kubeconfig to connect to the hub cluster
#   2. Retrieves and displays the ArgoCD URL and credentials
#   3. Sets up GitLab repository and SSH keys
#   4. Configures Git remote for the working repository
#   5. Creates a secret in ArgoCD for Git repository access
#   6. Logs in to ArgoCD CLI and lists applications
#
# USAGE:
#   ./1-argocd-gitlab-setup.sh
#
# PREREQUISITES:
#   - The management cluster must be created (run 0-initial-setup.sh first)
#   - Environment variables must be set:
#     - AWS_REGION: AWS region where resources are deployed
#     - WORKSPACE_PATH: Path to the workspace directory
#     - WORKING_REPO: Name of the working repository
#     - GIT_USERNAME: Git username for authentication
#     - IDE_PASSWORD: Password for ArgoCD and GitLab authentication
#
# SEQUENCE:
#   This is the second script (1) in the setup sequence.
#   Run after 0-initial-setup.sh and before 2-bootstrap-accounts.sh
#
#############################################################################

aws eks update-kubeconfig --name hub-cluster --alias hub-cluster

export DOMAIN_NAME=$(aws cloudfront list-distributions --query "DistributionList.Items[?contains(Origins.Items[0].Id, 'http-origin')].DomainName | [0]" --output text)
echo "ArgoCD URL: https://$DOMAIN_NAME/argocd
   Login: admin
   Password: $IDE_PASSWORD"


echo "Create Gitlab Git repository and secret for Argo CD to access the Git repository:"

export GITLAB_URL=https://$(aws cloudfront list-distributions --query "DistributionList.Items[?contains(Origins.Items[0].Id, 'gitlab')].DomainName | [0]" --output text)
export NLB_DNS=$(aws elbv2 describe-load-balancers --region $AWS_REGION --names gitlab --query 'LoadBalancers[0].DNSName' --output text) >> ~/environment/.envrc
echo "export GITLAB_URL=$GITLAB_URL" >> ~/environment/.envrc
echo "export NLB_DNS=$NLB_DNS" >> ~/environment/.envrc

$WORKSPACE_PATH/$WORKING_REPO/scripts/gitlab_create_keys.sh

cd $WORKSPACE_PATH/$WORKING_REPO
git remote add origin ssh://git@$NLB_DNS/$GIT_USERNAME/$WORKING_REPO.git

git push --set-upstream origin main

envsubst << 'EOF' | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
   name: git-${WORKING_REPO}
   namespace: argocd
   labels:
      argocd.argoproj.io/secret-type: repository
stringData:
   url: ${GITLAB_URL}/${GIT_USERNAME}/${WORKING_REPO}.git
   type: git
   username: $GIT_USERNAME
   password: $IDE_PASSWORD
EOF


# Login to ArgoCD CLI
argocd login --username admin --password $IDE_PASSWORD --grpc-web-root-path /argocd $DOMAIN_NAME

#List apps
argocd app list

kubectl get applications -n argocd

echo "ArgoCD and GitLab setup completed successfully."
echo "Next step: Run 2-bootstrap-accounts.sh to bootstrap management and spoke accounts."
