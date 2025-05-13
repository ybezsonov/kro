#!/bin/bash
set -e
# SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# export DOMAIN_NAME=$($SCRIPT_DIR/get_cloudfront.sh)
# export NLB_DNS=$($SCRIPT_DIR/get_nlb.sh)
# export GITLAB_URL=https://$DOMAIN_NAME/gitlab
export GITLAB_POD_NAME=$(kubectl get pods -n gitlab | grep Running | awk '{print $1}')
export ROOT_GITLAB_TOKEN=root-$IDE_PASSWORD

echo Updating GitLab settings ...
kubectl exec -it $GITLAB_POD_NAME -n gitlab -- gitlab-rails runner '::Gitlab::CurrentSettings.update!(signup_enabled: false)'

echo Creating GitLab API token for root ...
kubectl exec -it $GITLAB_POD_NAME -n gitlab -- gitlab-rails runner "
token = User.find_by_username('root').personal_access_tokens.create(
  name: 'initial root token',
  scopes: [
    'api',
    'read_user',
    'read_repository',
    'write_repository',
    'sudo',
    'admin_mode'
  ],
  expires_at: 365.days.from_now
)
token.set_token('${ROOT_GITLAB_TOKEN}')
token.save!
"

echo Creating $GIT_USERNAME ...
curl -sS -X 'POST' "$GITLAB_URL/api/v4/users" \
  -H "PRIVATE-TOKEN: $ROOT_GITLAB_TOKEN" \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d "{
  \"name\": \"$GIT_USERNAME\",
  \"username\": \"$GIT_USERNAME\",
  \"email\": \"$GIT_USERNAME@example.com\",
  \"password\": \"$IDE_PASSWORD\"
}" && echo -e "\n"

echo Creating GitLab API token for $GIT_USERNAME ...
kubectl exec -it $GITLAB_POD_NAME -n gitlab -- gitlab-rails runner "
token = User.find_by_username('$GIT_USERNAME').personal_access_tokens.create(
  name: 'initial $GIT_USERNAME token',
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

echo Adding an SSH key for $GIT_USERNAME ...
PUB_KEY=$(sudo cat /home/ec2-user/.ssh/id_rsa.pub)
TITLE="$(hostname)$(date +%s)"
USER_ID=$(curl -sS "$GITLAB_URL/api/v4/users?search=$GIT_USERNAME" -H "PRIVATE-TOKEN: $ROOT_GITLAB_TOKEN" | jq '.[0].id')
curl -sS -X 'POST' "$GITLAB_URL/api/v4/users/$USER_ID/keys" \
  -H "PRIVATE-TOKEN: $ROOT_GITLAB_TOKEN" \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d "{
  \"key\": \"$PUB_KEY\",
  \"title\": \"$TITLE\"
}" && echo -e "\n"

ssh-keyscan -H $NLB_DNS >> ~/.ssh/known_hosts

echo Creating eks-cluster-mgmt Git repository ...
curl -Ss -X 'POST' "$GITLAB_URL/api/v4/projects/" \
  -H "PRIVATE-TOKEN: $IDE_PASSWORD" \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d "{
  \"name\": \"$WORKING_REPO\"
}" && echo -e "\n"

echo "GitLab url: $GITLAB_URL"
echo "GitLab username: $GIT_USERNAME"
echo "GitLab password: $IDE_PASSWORD"
