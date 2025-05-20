#!/bin/bash

#############################################################################
# EKS Cluster Access Setup Script
#############################################################################
#
# DESCRIPTION:
#   This script automates the process of setting up access to all EKS clusters
#   defined in the values.yaml file. It performs the following operations:
#
#   1. Parses the values.yaml file to identify all EKS clusters
#   2. Verifies each cluster exists in the specified region
#   3. Adds the current user (ide-user) to each cluster's access entries
#   4. Associates the admin policy with the user for each cluster
#   5. Updates the kubeconfig file to enable kubectl access
#   6. Verifies connectivity to each cluster
#
# USAGE:
#   ./eks-cluster-access-setup.sh
#
# NOTES:
#   - This script is idempotent and can be run multiple times without errors
#   - It will skip clusters that don't exist or can't be accessed
#   - The script assumes the AWS CLI is configured with appropriate credentials
#
#############################################################################

# Set AWS_PAGER to empty to disable paging
export AWS_PAGER=""

# Define colors for better readability
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Path to values.yaml file
VALUES_FILE="/home/ec2-user/environment/eks-cluster-mgmt/fleet/kro-values/tenants/tenant1/kro-clusters/values.yaml"

# Function to check if a cluster exists
check_cluster_exists() {
    local cluster_name="$1"
    local region="$2"
    
    echo -e "${BLUE}Checking if cluster $cluster_name exists in region $region...${NC}"
    if aws eks describe-cluster --name "$cluster_name" --region "$region" &> /dev/null; then
        echo -e "${GREEN}Cluster $cluster_name exists in region $region.${NC}"
        return 0
    else
        echo -e "${RED}Cluster $cluster_name does not exist in region $region.${NC}"
        return 1
    fi
}

# Function to add ide-user to cluster access entries
add_ide_user_to_cluster() {
    local cluster_name="$1"
    local region="$2"
    
    echo -e "${BLUE}Adding ide-user to access entries for $cluster_name in $region...${NC}"
    
    # Check if access entry already exists
    if aws eks describe-access-entry --cluster-name "$cluster_name" --principal-arn "arn:aws:iam::862416928860:role/ide-user" --region "$region" &> /dev/null; then
        echo -e "${YELLOW}Access entry for ide-user already exists in $cluster_name${NC}"
        
        # Check if admin policy is associated
        if ! aws eks list-associated-access-policies --cluster-name "$cluster_name" --principal-arn "arn:aws:iam::862416928860:role/ide-user" --region "$region" | grep -q "AmazonEKSClusterAdminPolicy"; then
            echo "Associating admin policy with ide-user..."
            aws eks associate-access-policy \
                --cluster-name "$cluster_name" \
                --principal-arn "arn:aws:iam::862416928860:role/ide-user" \
                --policy-arn "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy" \
                --access-scope type=cluster \
                --region "$region"
        else
            echo -e "${YELLOW}Admin policy already associated with ide-user for $cluster_name${NC}"
        fi
    else
        # Create access entry
        echo "Creating access entry for ide-user..."
        aws eks create-access-entry \
            --cluster-name "$cluster_name" \
            --principal-arn "arn:aws:iam::862416928860:role/ide-user" \
            --region "$region"
        
        # Associate admin policy
        echo "Associating admin policy with ide-user..."
        aws eks associate-access-policy \
            --cluster-name "$cluster_name" \
            --principal-arn "arn:aws:iam::862416928860:role/ide-user" \
            --policy-arn "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy" \
            --access-scope type=cluster \
            --region "$region"
    fi
}

# Function to update kubeconfig for a cluster
update_kubeconfig() {
    local cluster_name="$1"
    local region="$2"
    
    echo -e "${BLUE}Updating kubeconfig for $cluster_name...${NC}"
    aws eks update-kubeconfig --name "$cluster_name" --region "$region" --alias "$cluster_name"
    
    # Verify connection
    echo -e "${BLUE}Verifying connection to $cluster_name...${NC}"
    if kubectl --context="$cluster_name" get nodes &> /dev/null; then
        echo -e "${GREEN}Successfully connected to $cluster_name${NC}"
        return 0
    else
        echo -e "${RED}Failed to connect to $cluster_name${NC}"
        return 1
    fi
}

# Function to connect to a cluster and get information
connect_to_cluster() {
    local cluster_name="$1"
    local account_id="$2"
    local region="$3"
    
    echo -e "\n${GREEN}=========================================================${NC}"
    echo -e "${GREEN}Connecting to cluster: $cluster_name${NC}"
    echo -e "${GREEN}AWS Account ID: $account_id${NC}"
    echo -e "${GREEN}Region: $region${NC}"
    echo -e "${GREEN}=========================================================${NC}"
    
    # Check if cluster exists
    if ! check_cluster_exists "$cluster_name" "$region"; then
        echo -e "${RED}Skipping $cluster_name as it does not exist.${NC}"
        return 1
    fi
    
    # Add ide-user to cluster access entries
    add_ide_user_to_cluster "$cluster_name" "$region"
    
    # Update kubeconfig and verify connection
    if ! update_kubeconfig "$cluster_name" "$region"; then
        echo -e "${RED}Could not establish connection to $cluster_name. Skipping further operations.${NC}"
        return 1
    fi
    
    # Get cluster nodes
    echo -e "${BLUE}Nodes in $cluster_name:${NC}"
    kubectl --context="$cluster_name" get nodes
    
    # Get services in all namespaces
    echo -e "${BLUE}Services in $cluster_name:${NC}"
    kubectl --context="$cluster_name" get svc -A
    
    echo -e "${GREEN}Successfully connected to $cluster_name${NC}"
    return 0
}

# Function to parse values.yaml and extract cluster information
parse_values_yaml() {
    # Check if values.yaml exists
    if [ ! -f "$VALUES_FILE" ]; then
        echo -e "${RED}Values file not found: $VALUES_FILE${NC}"
        exit 1
    fi
    
    echo -e "${BLUE}Parsing values.yaml to extract cluster information...${NC}"
    
    # Extract cluster information using grep and awk
    # This is a simple parser and might not work for all YAML structures
    local in_clusters_section=false
    local current_cluster=""
    local account_id=""
    local region=""
    
    while IFS= read -r line; do
        # Check if we're in the clusters section
        if [[ "$line" =~ ^clusters: ]]; then
            in_clusters_section=true
            continue
        fi
        
        # Skip commented lines
        if [[ "$line" =~ ^[[:space:]]*# ]]; then
            continue
        fi
        
        # Check if we're in a cluster definition
        if [[ "$in_clusters_section" == true && "$line" =~ ^[[:space:]]+([^:]+): ]]; then
            current_cluster="${BASH_REMATCH[1]}"
            continue
        fi
        
        # Extract accountId
        if [[ "$current_cluster" != "" && "$line" =~ [[:space:]]+accountId:[[:space:]]+\"([^\"]+)\" ]]; then
            account_id="${BASH_REMATCH[1]}"
        fi
        
        # Extract region
        if [[ "$current_cluster" != "" && "$line" =~ [[:space:]]+region:[[:space:]]+\"([^\"]+)\" ]]; then
            region="${BASH_REMATCH[1]}"
            
            # If we have all the information, connect to the cluster
            if [[ "$current_cluster" != "" && "$account_id" != "" && "$region" != "" ]]; then
                connect_to_cluster "$current_cluster" "$account_id" "$region"
                current_cluster=""
                account_id=""
                region=""
            fi
        fi
    done < "$VALUES_FILE"
}

# Main script execution
echo -e "${GREEN}Starting EKS cluster access setup...${NC}"
echo -e "${YELLOW}This script will connect to all clusters defined in values.yaml${NC}"
echo -e "${YELLOW}It is idempotent and can be run multiple times without errors${NC}"

# Parse values.yaml and connect to clusters
parse_values_yaml

echo -e "${GREEN}EKS cluster access setup completed.${NC}"
