#!/bin/bash
#########################################################################
# Script: 6-tools-urls.sh
# Description: Displays URLs and credentials for all tools deployed in the
#              EKS cluster management environment with perfect table alignment
# Author: AWS
# Date: 2025-05-20
# Usage: ./6-tools-urls.sh
#########################################################################

# Define color codes
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
BOLD='\033[1m'
UNDERLINE='\033[4m'
NC='\033[0m' # No Color

# Switch to hub cluster context
echo -e "${BLUE}Switching to hub-cluster context...${NC}"
kctx hub-cluster

# Get CloudFront domain name
echo -e "${BLUE}Retrieving CloudFront domain...${NC}"
DOMAIN_NAME=$(aws cloudfront list-distributions --query "DistributionList.Items[?contains(Origins.Items[0].Id, 'http-origin')].DomainName | [0]" --output text)

# Get Kargo URL
echo -e "${BLUE}Retrieving Kargo URL...${NC}"
export KARGO_URL=http://$(kubectl get svc kargo-api -n kargo -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')

# Get GitLab URL
echo -e "${BLUE}Retrieving GitLab URL...${NC}"
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
echo -e "\n${BOLD}${UNDERLINE}EKS Cluster Management Tools${NC}\n"

# Print table header with ASCII characters
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}| ${CYAN}$(pad_string "Tool" $TOOL_COL)${NC}${BOLD} | ${CYAN}$(pad_string "URL" $URL_COL)${NC}${BOLD} | ${CYAN}$(pad_string "Credentials" $CRED_COL)${NC}${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"

# Print table rows with exact character counts
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "ArgoCD" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$ARGOCD_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "admin / $IDE_PASSWORD" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}|${NC} $(pad_string "" $TOOL_COL)${NC}${BOLD} |${NC} $(pad_string "" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "or SSO: user1 / $IDE_PASSWORD" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "Keycloak" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$KEYCLOAK_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "admin / $IDE_PASSWORD" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "Backstage" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$BACKSTAGE_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "SSO: user1 / $IDE_PASSWORD" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "Argo-Workflows" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$WORKFLOWS_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "SSO: user1 / $IDE_PASSWORD" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "Gitlab" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$GITLAB_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "root / $IDE_PASSWORD" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}|${NC} $(pad_string "" $TOOL_COL)${NC}${BOLD} |${NC} $(pad_string "" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "user1 / $IDE_PASSWORD" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"
echo -e "${BOLD}|${NC} ${GREEN}$(pad_string "Kargo" $TOOL_COL)${NC}${BOLD} |${NC} ${YELLOW}$(pad_string "$KARGO_URL" $URL_COL)${NC}${BOLD} |${NC} $(pad_string "$IDE_PASSWORD" $CRED_COL)${BOLD} |${NC}"
echo -e "${BOLD}+----------------+-------------------------------------------------------+-------------------------------------+${NC}"

# Print full URLs for easy copy-paste
echo -e "\n${BOLD}${UNDERLINE}Full URLs for copy-paste:${NC}"
echo -e "${CYAN}ArgoCD:${NC} $ARGOCD_URL"
echo -e "${CYAN}Keycloak:${NC} $KEYCLOAK_URL"
echo -e "${CYAN}Backstage:${NC} $BACKSTAGE_URL"
echo -e "${CYAN}Argo-Workflows:${NC} $WORKFLOWS_URL"
echo -e "${CYAN}GitLab:${NC} $GITLAB_URL"
echo -e "${CYAN}Kargo:${NC} $KARGO_URL"

# Print usage instructions
echo -e "\n${BOLD}${UNDERLINE}Usage examples:${NC}"
echo -e "  ${CYAN}ArgoCD CLI login:${NC} argocd login --username admin --password $IDE_PASSWORD --grpc-web-root-path /argocd $DOMAIN_NAME"
echo -e "  ${CYAN}View applications:${NC} argocd app list"
echo -e "  ${CYAN}Sync application:${NC} argocd app sync <app-name>"
echo -e "  ${CYAN}Access Kargo:${NC} curl --head -X GET $KARGO_URL"
