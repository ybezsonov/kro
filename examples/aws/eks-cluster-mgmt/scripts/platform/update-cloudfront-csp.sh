#!/bin/bash

# Script to create and apply a CloudFront Response Headers Policy for frame-src
# This addresses Content Security Policy issues with iframes

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Get CloudFront domain and distribution ID using the existing script
CLOUDFRONT_DOMAIN=$($SCRIPT_DIR/get_cloudfront.sh)
echo "Found CloudFront domain: $CLOUDFRONT_DOMAIN"

# Get the distribution ID
DISTRIBUTIONS=$(aws cloudfront list-distributions --output json)
NLB_DNS=$($SCRIPT_DIR/get_nlb.sh)

DISTRIBUTION_ID=""
for ID in $(echo "$DISTRIBUTIONS" | jq -r '.DistributionList.Items[].Id'); do
    ORIGINS=$(aws cloudfront get-distribution --id "$ID" --query "Distribution.DistributionConfig.Origins.Items[*].DomainName" --output json)

    if echo "$ORIGINS" | jq -r '.[]' 2>/dev/null | grep -q "$NLB_DNS"; then
        DISTRIBUTION_ID="$ID"
        break
    fi
done

if [ -z "$DISTRIBUTION_ID" ]; then
    echo "Error: Could not find CloudFront distribution ID"
    exit 1
fi

echo "Found CloudFront distribution ID: $DISTRIBUTION_ID"

# Create a unique policy name with timestamp to avoid conflicts
POLICY_NAME="AllowFrameSrcPolicy-$(date +%Y%m%d%H%M%S)"

echo "Creating CloudFront Response Headers Policy..."
POLICY_CONFIG=$(mktemp)
cat > $POLICY_CONFIG << EOF
{
  "Name": "$POLICY_NAME",
  "Comment": "Policy to allow frame-src for CloudFront domain",
  "SecurityHeadersConfig": {
    "ContentSecurityPolicy": {
      "Override": true,
      "ContentSecurityPolicy": "frame-src 'self' https://$CLOUDFRONT_DOMAIN http://$CLOUDFRONT_DOMAIN:443;"
    }
  }
}
EOF

POLICY_RESULT=$(aws cloudfront create-response-headers-policy \
  --response-headers-policy-config file://$POLICY_CONFIG)

# Extract the policy ID from the result
POLICY_ID=$(echo $POLICY_RESULT | jq -r '.ResponseHeadersPolicy.Id')

echo "Created Response Headers Policy with ID: $POLICY_ID"

# Get the current ETag and distribution config
DISTRIBUTION_INFO=$(aws cloudfront get-distribution --id $DISTRIBUTION_ID)
ETAG=$(echo $DISTRIBUTION_INFO | jq -r '.ETag')

echo "Retrieved ETag: $ETAG for distribution $DISTRIBUTION_ID"

# Get the current distribution config
TEMP_CONFIG_FILE=$(mktemp)
aws cloudfront get-distribution-config --id $DISTRIBUTION_ID > $TEMP_CONFIG_FILE

# Extract and modify the distribution config
DISTRIBUTION_CONFIG=$(jq '.DistributionConfig' $TEMP_CONFIG_FILE)

# Update the DefaultCacheBehavior to include the ResponseHeadersPolicyId
UPDATED_CONFIG=$(echo $DISTRIBUTION_CONFIG | jq '.DefaultCacheBehavior.ResponseHeadersPolicyId = "'"$POLICY_ID"'"')

# Create a temporary file for the updated config
UPDATED_CONFIG_FILE=$(mktemp)
echo $UPDATED_CONFIG > $UPDATED_CONFIG_FILE

echo "Updating CloudFront distribution to use the new policy..."
aws cloudfront update-distribution \
  --id $DISTRIBUTION_ID \
  --distribution-config file://$UPDATED_CONFIG_FILE \
  --if-match $ETAG

# Clean up temporary files
rm $TEMP_CONFIG_FILE $UPDATED_CONFIG_FILE

echo "CloudFront distribution updated successfully!"
echo "Note: It may take up to 15-30 minutes for the changes to propagate globally."
