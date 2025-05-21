#!/bin/bash
#########################################################################
# Script: 6-tools-urls.sh
# Description: Displays URLs and credentials for all tools deployed in the
#              EKS cluster management environment with perfect table alignment
# Author: AWS
# Date: 2025-05-20
# Usage: ./6-tools-urls.sh
#########################################################################

# Source the colors script
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/colors.sh"

# Switch to hub cluster context
print_step "Switching to hub-cluster context..."
kubectx hub-cluster

# Get CloudFront domain name
print_info "Retrieving CloudFront domain..."
DOMAIN_NAME=$(aws cloudfront list-distributions --query "DistributionList.Items[?contains(Origins.Items[0].Id, 'http-origin')].DomainName | [0]" --output text)

# Get Kargo URL
print_info "Retrieving Kargo URL..."
export KARGO_URL=http://$(kubectl get svc kargo-api -n kargo -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')

# Get GitLab URL
print_info "Retrieving GitLab URL..."
GITLAB_URL=https://$(aws cloudfront list-distributions --query "DistributionList.Items[?contains(Origins.Items[0].Id, 'gitlab')].DomainName | [0]" --output text)

# Define fixed column widths
TOOL_COL=14
URL_COL=55
CRED_COL=35

# Function to create a padded string of specified length
pad_string() {
    local str="$1"
    local len=$2
    printf "%-${len}s" "$str"
}

# Store URLs for display
ARGOCD_URL="https://$DOMAIN_NAME/argocd"
KEYCLOAK_URL="https://$DOMAIN_NAME/keycloak"
BACKSTAGE_URL="https://$DOMAIN_NAME"
WORKFLOWS_URL="https://$DOMAIN_NAME/argo-workflows"

# Print header
print_header "EKS Cluster Management Tools"

# Print table header with ASCII characters
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}| ${CYAN}$(pad_string "Tool" $TOOL_COL)${NC}${BOLD} | ${CYAN}$(pad_string "URL" $URL_COL)${NC}${BOLD} | ${CYAN}$(pad_string "Credentials" $CRED_COL)${NC}${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"

# Print table rows with exact character counts
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "ArgoCD" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$ARGOCD_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "admin" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}|${NC} $(pad_string "" $TOOL_COL)${NC}${BOLD} |${NC} $(pad_string "" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "or SSO: user1" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "Keycloak" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$KEYCLOAK_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "admin" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "Backstage" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$BACKSTAGE_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "SSO: user1" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "Argo-Workflows" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$WORKFLOWS_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "SSO: user1" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "Gitlab" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$GITLAB_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "root" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}|${NC} $(pad_string "" $TOOL_COL)${NC}${BOLD} |${NC} $(pad_string "" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "user1" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "Kargo" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$KARGO_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "\$IDE_PASSWORD" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"

# Print full URLs for easy copy-paste
print_header "Full URLs for copy-paste"
print_info "ArgoCD: ${BOLD}$ARGOCD_URL${NC}"
print_info "Keycloak: ${BOLD}$KEYCLOAK_URL${NC}"
print_info "Backstage: ${BOLD}$BACKSTAGE_URL${NC}"
print_info "Argo-Workflows: ${BOLD}$WORKFLOWS_URL${NC}"
print_info "GitLab: ${BOLD}$GITLAB_URL${NC}"
print_info "Kargo: ${BOLD}$KARGO_URL${NC}"

# Print usage instructions
print_header "Usage examples"
print_info "ArgoCD CLI login: ${BOLD}argocd login --username admin --password $IDE_PASSWORD --grpc-web-root-path /argocd $DOMAIN_NAME${NC}"
print_info "View applications: ${BOLD}argocd app list${NC}"
print_info "Sync application: ${BOLD}argocd app sync <app-name>${NC}"
print_info "Access Kargo: ${BOLD}curl --head -X GET $KARGO_URL${NC}"

# Print usage instructions
print_header "Password"
print_step "$IDE_PASSWORD"