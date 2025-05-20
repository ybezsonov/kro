#!/bin/bash

#############################################################################
# Configure EKS Cluster Access
#############################################################################
#
# DESCRIPTION:
#   This script configures access to the EKS clusters created in the previous
#   steps. It:
#   1. Creates access entries for the ide-user role in each EKS cluster
#   2. Associates the AmazonEKSClusterAdminPolicy with the ide-user role
#   3. Updates the kubeconfig file to enable kubectl access
#
# USAGE:
#   ./5-cluster-access.sh
#
# PREREQUISITES:
#   - The Argo Rollouts demo application must be deployed (run 4-deploy-argo-rollouts-demo.sh first)
#   - AWS CLI must be configured with appropriate credentials
#
# SEQUENCE:
#   This is the final script (5) in the setup sequence.
#   Run after 4-deploy-argo-rollouts-demo.sh
#
# NEXT STEPS:
#   After running this script, you can use the following scripts to:
#   - eks-cluster-access-setup.sh: Configure access to all EKS clusters
#   - multi-cluster-dashboard-generator.sh: Generate a dashboard for all clusters
#
#############################################################################

# Run the eks-cluster-access-setup.sh script to configure access to all clusters
./eks-cluster-access-setup.sh

# Generate a dashboard for all clusters
./multi-cluster-dashboard-generator.sh

echo "EKS cluster access configuration completed."
echo "You can now access all clusters using kubectl."
echo "The multi-cluster dashboard is available at: /home/ec2-user/environment/eks-cluster-mgmt/scripts/dashboard.html"
