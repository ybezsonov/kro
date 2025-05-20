#!/bin/bash

#############################################################################
# Bootstrap Management and Spoke Accounts
#############################################################################
#
# DESCRIPTION:
#   This script bootstraps the management and spoke AWS accounts for EKS
#   cluster management. It:
#   1. Creates ACK workload roles with the current user added
#   2. Monitors ResourceGraphDefinitions until they are all in Active state
#   3. Restarts the KRO deployment if needed to activate resources
#
# USAGE:
#   ./2-bootstrap-accounts.sh
#
# PREREQUISITES:
#   - ArgoCD and GitLab must be set up (run 1-argocd-gitlab-setup.sh first)
#   - The create_ack_workload_roles.sh script must be available
#   - kubectl must be configured to access the hub cluster
#
# SEQUENCE:
#   This is the third script (2) in the setup sequence.
#   Run after 1-argocd-gitlab-setup.sh and before 3-create-spoke-clusters.sh
#
#############################################################################

echo "Bootstrapping Management/Spoke accounts"

cd $WORKSPACE_PATH/$WORKING_REPO/scripts
./create_ack_workload_roles.sh


echo "Checking ResourceGraphDefinitions status..."

while true; do
  # Get all ResourceGraphDefinitions and check if any are not in Active state
  inactive_rgds=$(kubectl get resourcegraphdefinitions.kro.run -o jsonpath='{.items[?(@.status.state!="Active")].metadata.name}')
  
  if [ -z "$inactive_rgds" ]; then
    echo "All ResourceGraphDefinitions are in Active state!"
    break
  else
    echo "Found ResourceGraphDefinitions not in Active state: $inactive_rgds"
    echo "Restarting kro deployment in kro-system namespace..."
    kubectl rollout restart deployment -n kro-system kro
    echo "Waiting 30 seconds before checking again..."
    sleep 30
  fi
done

echo "Account bootstrapping completed successfully."
echo "Next step: Run 3-create-spoke-clusters.sh to create the spoke EKS clusters."
