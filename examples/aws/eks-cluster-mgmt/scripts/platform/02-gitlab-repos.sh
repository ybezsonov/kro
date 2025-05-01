#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export DOMAIN_NAME=$($SCRIPT_DIR/get_cloudfront.sh)
export NLB_DNS=$($SCRIPT_DIR/get_nlb.sh)
export GITLAB_URL=https://$DOMAIN_NAME/gitlab
export GITLAB_POD_NAME=$(kubectl get pods -n gitlab | grep Running | awk '{print $1}')
export ROOT_GITLAB_TOKEN=root-$IDE_PASSWORD

echo Creating user1 ...
curl -sS -X 'POST' "$GITLAB_URL/api/v4/users" \
  -H "PRIVATE-TOKEN: $ROOT_GITLAB_TOKEN" \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d "{
  \"name\": \"User1\",
  \"username\": \"user1\",
  \"email\": \"user1@example.com\",
  \"password\": \"$IDE_PASSWORD\"
}"

echo Creating GitLab API token for user1 ...
kubectl exec -it $GITLAB_POD_NAME -n gitlab -- gitlab-rails runner "
token = User.find_by_username('user1').personal_access_tokens.create(
  name: 'initial user1 token',
  scopes: [
    'api',
    'read_user',
    'read_repository',
    'write_repository'
  ],
  expires_at: 365.days.from_now
)
token.set_token('${IDE_PASSWORD}')
token.save!
"

echo Adding an SSH key for user1 ...
PUB_KEY=$(sudo cat /home/ec2-user/.ssh/id_rsa.pub)
TITLE="$(hostname)$(date +%s)"
USER_ID=$(curl -sS "$GITLAB_URL/api/v4/users?search=user1" -H "PRIVATE-TOKEN: $ROOT_GITLAB_TOKEN" | jq '.[0].id')
curl -sS -X 'POST' "$GITLAB_URL/api/v4/users/$USER_ID/keys" \
  -H "PRIVATE-TOKEN: $ROOT_GITLAB_TOKEN" \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d "{
  \"key\": \"$PUB_KEY\",
  \"title\": \"$TITLE\"
}"

git config --global user.email "user1@example.com"
git config --global user.name "User1"

ssh-keyscan -H $NLB_DNS >> ~/.ssh/known_hosts

echo Creating eks-cluster-mgmt Git repository ...
curl -Ss -X 'POST' "$GITLAB_URL/api/v4/projects/" \
  -H "PRIVATE-TOKEN: $IDE_PASSWORD" \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d "{
  \"name\": \"eks-cluster-mgmt\"
}"

echo "GitLab url: $GITLAB_URL"
echo "GitLab username: user1"
echo "GitLab password: $IDE_PASSWORD"

git clone ssh://git@$(./get_nlb.sh)/$user1/$eks-cluster-mgmt.git e2
