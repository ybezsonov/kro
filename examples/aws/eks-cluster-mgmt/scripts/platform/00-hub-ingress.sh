#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
NLB_DNS=$($SCRIPT_DIR/get_nlb.sh)

# Create a temporary JSON file for the CloudFront distribution configuration
TEMP_CONFIG=$(mktemp)

cat > "$TEMP_CONFIG" << EOF
{
    "CallerReference": "hub-ingress-$(date +%s)",
    "Comment": "CloudFront distribution for hub-ingress NLB",
    "Origins": {
        "Quantity": 1,
        "Items": [
            {
                "Id": "http-origin",
                "DomainName": "$NLB_DNS",
                "CustomOriginConfig": {
                    "HTTPPort": 80,
                    "HTTPSPort": 443,
                    "OriginProtocolPolicy": "http-only",
                    "OriginSslProtocols": {
                        "Quantity": 1,
                        "Items": ["TLSv1.2"]
                    },
                    "OriginReadTimeout": 30,
                    "OriginKeepaliveTimeout": 5
                }
            }
        ]
    },
    "DefaultCacheBehavior": {
        "TargetOriginId": "http-origin",
        "ViewerProtocolPolicy": "redirect-to-https",
        "AllowedMethods": {
            "Quantity": 7,
            "Items": ["GET", "HEAD", "POST", "PUT", "PATCH", "OPTIONS", "DELETE"],
            "CachedMethods": {
                "Quantity": 2,
                "Items": ["GET", "HEAD"]
            }
        },
        "CachePolicyId": "4135ea2d-6df8-44a3-9df3-4b5a84be39ad",
        "OriginRequestPolicyId": "216adef6-5c7f-47e4-b989-5492eafa07d3",
        "Compress": false,
        "LambdaFunctionAssociations": {
            "Quantity": 0
        },
        "FunctionAssociations": {
            "Quantity": 0
        }
    },
    "CacheBehaviors": {
        "Quantity": 0
    },
    "CustomErrorResponses": {
        "Quantity": 0
    },
    "Enabled": true,
    "PriceClass": "PriceClass_All",
    "ViewerCertificate": {
        "CloudFrontDefaultCertificate": true,
        "MinimumProtocolVersion": "TLSv1",
        "CertificateSource": "cloudfront"
    },
    "Restrictions": {
        "GeoRestriction": {
            "RestrictionType": "none",
            "Quantity": 0
        }
    },
    "HttpVersion": "http2",
    "IsIPV6Enabled": true
}
EOF

DISTRIBUTION_ID=$(aws cloudfront create-distribution --distribution-config file://"$TEMP_CONFIG" --query "Distribution.Id" --output text)

rm "$TEMP_CONFIG"

# Get the domain name of the created distribution
DOMAIN_NAME=$(aws cloudfront get-distribution --id "$DISTRIBUTION_ID" --query "Distribution.DomainName" --output text)

echo "NLB Domain Name: $NLB_DNS"
echo "Cloudfront Domain Name: $DOMAIN_NAME"
