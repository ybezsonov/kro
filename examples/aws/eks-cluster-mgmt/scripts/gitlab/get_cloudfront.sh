#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
NLB_DNS=$($SCRIPT_DIR/get_nlb.sh)
DISTRIBUTIONS=$(aws cloudfront list-distributions --output json)

DISTRIBUTION_ID=""
DOMAIN_NAME=""

for ID in $(echo "$DISTRIBUTIONS" | jq -r '.DistributionList.Items[].Id'); do
    ORIGINS=$(aws cloudfront get-distribution --id "$ID" --query "Distribution.DistributionConfig.Origins.Items[*].DomainName" --output json)

    if echo "$ORIGINS" | jq -r '.[]' 2>/dev/null | grep -q "$NLB_DNS"; then
        DISTRIBUTION_ID="$ID"
        export DOMAIN_NAME=$(aws cloudfront get-distribution --id "$DISTRIBUTION_ID" --query "Distribution.DomainName" --output text)
        break
    fi
done

echo $DOMAIN_NAME
