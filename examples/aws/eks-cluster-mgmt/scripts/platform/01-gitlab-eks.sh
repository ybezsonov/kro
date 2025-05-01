#!/bin/bash
set -e
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export DOMAIN_NAME=$($SCRIPT_DIR/get_cloudfront.sh)

echo Deploying GitLab to $DOMAIN_NAME...
envsubst < $SCRIPT_DIR/manifests/gitlab.yaml | kubectl apply -f -
kubectl wait deployment gitlab -n gitlab --for condition=Available=True --timeout=600s

GITLAB_POD_NAME=$(kubectl get pods -n gitlab | grep Running | awk '{print $1}')
echo GITLAB_POD_NAME is $GITLAB_POD_NAME

echo Updating GitLab settings ...
kubectl exec -it $GITLAB_POD_NAME -n gitlab -- gitlab-rails runner '::Gitlab::CurrentSettings.update!(signup_enabled: false)'

echo Creating GitLab API token for root ...
export ROOT_GITLAB_TOKEN=root-$IDE_PASSWORD
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

echo "GitLab url: https://${DOMAIN_NAME}/gitlab"
echo "GitLab username: root"
echo "GitLab password: $IDE_PASSWORD"
