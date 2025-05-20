#!/bin/bash

#############################################################################
# Initial Setup Script for EKS Cluster Management
#############################################################################
#
# DESCRIPTION:
#   This script performs the initial setup for the EKS cluster management
#   environment. It:
#   1. Creates a local Git repository for EKS cluster management
#   2. Copies example files from the KRO repository
#   3. Updates configuration with the management account ID
#   4. Creates the management cluster using Terraform
#
# USAGE:
#   ./0-initial-setup.sh
#
# PREREQUISITES:
#   - Environment variables must be set:
#     - KRO_REPO_URL: URL of the KRO repository
#     - KRO_REPO_BRANCH: Branch to use from the KRO repository
#     - WORKSPACE_PATH: Path to the workspace directory
#     - WORKING_REPO: Name of the working repository
#     - GIT_USERNAME: Git username for commits
#     - MGMT_ACCOUNT_ID: AWS Management account ID
#
# SEQUENCE:
#   This is the first script (0) in the setup sequence.
#   After running this script, proceed with 1-argocd-gitlab-setup.sh
#
#############################################################################

set -e
set -x

echo "Create eks-cluster-mgmt Git repository"
mkdir $WORKSPACE_PATH/$WORKING_REPO || true
cd $WORKSPACE_PATH/$WORKING_REPO
git config --global user.email "$GIT_USERNAME@example.com"
git config --global user.name "$GIT_USERNAME"

git init -b main

cp -r $WORKSPACE_PATH/kro/examples/aws/eks-cluster-mgmt/* $WORKSPACE_PATH/$WORKING_REPO/

git add .
git commit -q -m "initial commit" || true

echo "Creating the Management cluster"

# Update the `terraform.tfvars` with your values
# configure `accounts_ids` with the list of AWS accounts you want to use for spoke clusters. 
# If you want to create spoke clusters in the same management account, it just needs the management account id. 
# This parameter is used for IAM roles configuration.

sed -i "s|account_ids = \".*\"|account_ids = \"$MGMT_ACCOUNT_ID\"|" "$WORKSPACE_PATH/$WORKING_REPO/terraform/hub/terraform.tfvars"
/usr/lib/code-server/bin/code-server $WORKSPACE_PATH/$WORKING_REPO/terraform/hub/terraform.tfvars

cd $WORKSPACE_PATH/$WORKING_REPO/
git status
git add .
git commit -m "Terraform values"

cd $WORKSPACE_PATH/$WORKING_REPO/terraform/hub
./install.sh

echo "Initial setup completed successfully."
echo "Next step: Run 1-argocd-gitlab-setup.sh to configure ArgoCD and GitLab."
