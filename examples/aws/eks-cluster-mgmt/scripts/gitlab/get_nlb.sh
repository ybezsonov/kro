#!/bin/bash
set -e

NLB_PREFIX="${1:-hub-ingress}"
NLB_ARN=$(aws elbv2 describe-load-balancers --query "LoadBalancers[?starts_with(LoadBalancerName, '$NLB_PREFIX')].LoadBalancerArn" --output text)
export NLB_DNS=$(aws elbv2 describe-load-balancers --load-balancer-arns "$NLB_ARN" --query "LoadBalancers[0].DNSName" --output text)
echo $NLB_DNS
