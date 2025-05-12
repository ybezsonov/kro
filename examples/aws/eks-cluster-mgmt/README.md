# Amazon EKS multi cluster management using kro & ACK and continuous promotion to multiple environments using Argo CD, Kargo and Argo Rollouts

## Overview of a solution

This solution demonstrates:
- How to manage a fleet of [Amazon Elastic Kubernetes Service (Amazon EKS)](https://aws.amazon.com/eks/) clusters using [kro](https://kro.run/), [ACK](https://aws-controllers-k8s.github.io/community/), and [Argo CD]() across multiple regions and accounts by creating Amazon EKS clusters, and bootstraps them with the required add-ons.
- How to create continuous application promotion process to multiple environments (test, pre-prod, prod-eu, prod-us) using Argo CD, [Kargo](https://kargo.io/) and [Argo Rollouts](https://argoproj.github.io/rollouts/).

A hub-spoke model is used in this example; a management cluster (hub) is created as part of the initial setup and the controllers needed for provisioning and
bootstrapping workload clusters (spokes) are installed on top.

![EKS cluster management using kro & ACK](docs/eks-cluster-mgmt-central.drawio.png)

## Prerequisites

1. AWS account for the management cluster, and optional AWS accounts for spoke clusters (you can reuse management account for spoke too).

2. Deploy VSCode IDE. In additional to IDE it gives you pre-deployed Git Server with [Gitea](https://about.gitea.com/) for GitOps:

```sh
curl https://raw.githubusercontent.com/aws-samples/java-on-aws/main/infrastructure/cfn/ide-gitea-stack.yaml > ide-stack.yaml
CFN_S3=cfn-$(uuidgen | tr -d - | tr '[:upper:]' '[:lower:]')
aws s3 mb s3://$CFN_S3
aws cloudformation deploy --stack-name ide-stack \
    --template-file ./ide-stack.yaml \
    --s3-bucket $CFN_S3 \
    --capabilities CAPABILITY_NAMED_IAM
aws cloudformation describe-stacks --stack-name ide-stack --query "Stacks[0].Outputs[?OutputKey=='IdeUrl'].OutputValue" --output text
aws cloudformation describe-stacks --stack-name ide-stack --query "Stacks[0].Outputs[?OutputKey=='IdePassword'].OutputValue" --output text
```

3. Login to VSCode IDE using `IdeUrl` and `IdePassword` from the outputs above.

## Walkthrough

### Configuring workspace

1. Set variables

```sh
env | grep GITEA
export KRO_REPO_URL="https://github.com/ybezsonov/kro.git"
export KRO_REPO_BRANCH="app-promo"
export WORKING_REPO="eks-cluster-mgmt" # If you can avoid changing this, as you'll need to update in both terraform and gitops configurations
export TF_VAR_FILE="terraform.tfvars" # the name of terraform configuration file to use
export MGMT_ACCOUNT_ID=$(aws sts get-caller-identity --output text --query Account) # Or update to the AWS account to use for your management cluster
export WORKSPACE_PATH="$HOME/environment" # the directory where repos will be cloned e.g. ~/environment
export GITHUB_ORG_NAME=$GITEA_USERNAME # your Github/Gitea User-name or Organization you want to use for the work
```

2. Clone kro repository:

```sh
git clone $KRO_REPO_URL $WORKSPACE_PATH/kro
git -C $WORKSPACE_PATH/kro checkout $KRO_REPO_BRANCH
```

3. Create `eks-cluster-mgmt` GitOps repository:

```sh
curl -X 'POST' \
   "http://$GITEA_USERNAME:$IDE_PASSWORD@localhost:9000/api/v1/user/repos" \
   -H 'accept: application/json' \
   -H 'Content-Type: application/json' \
   -d "{
      \"name\": \"${WORKING_REPO}\"
   }"
```

4. Clone the working empty git repository:

```sh
git clone ssh://git@$GIT_SSH_ENDPOINT/$GITEA_USERNAME/$WORKING_REPO.git $WORKSPACE_PATH/$WORKING_REPO
```

5. Populate the repository:

```sh
cp -r $WORKSPACE_PATH/kro/examples/aws/eks-cluster-mgmt/* $WORKSPACE_PATH/$WORKING_REPO/
```

6. Replace Git Urls in the project:

```sh
find "$WORKSPACE_PATH/$WORKING_REPO/" -type f -exec sed -i'' -e "s|GIT_CLUSTER_MGMT_URL|${GITEA_EXTERNAL_URL}|g" {} +
find "$WORKSPACE_PATH/$WORKING_REPO/" -type f -exec sed -i'' -e "s|GIT_ORG_NAME|${GITEA_USERNAME}|g" {} +
```

7. Add, Commit and Push:

```sh
cd $WORKSPACE_PATH/$WORKING_REPO/
git status
git add .
git commit -m "initial commit"
git push
```

### Creating the Management cluster

1. Update the `terraform.tfvars` with your values:

- configure `accounts_ids` with the list of AWS accounts you want to use for spoke clusters. If you want to create spoke clusters in the same management account, just put the management account id. This parameter is used for IAM roles configuration.

```sh
sed -i "s|account_ids = \".*\"|account_ids = \"$MGMT_ACCOUNT_ID\"|" "$WORKSPACE_PATH/$WORKING_REPO/terraform/hub/terraform.tfvars"
code $WORKSPACE_PATH/$WORKING_REPO/terraform/hub/terraform.tfvars
```

2. Add, Commit and Push:

```sh
cd $WORKSPACE_PATH/$WORKING_REPO/
git status
git add .
git commit -m "Terraform values"
git push
```

3. Apply the terraform to create the management cluster:

```sh
cd $WORKSPACE_PATH/$WORKING_REPO/terraform/hub
./install.sh
```

4. Review the changes and accept to deploy.

> Wait until the deployment is successful. It should take around 15-20 minutes.

5. Connect to the cluster:

```sh
aws eks update-kubeconfig --name hub-cluster
```

5. Connect to the Argo CD UI. Execute the following command to get the Argo CD UI url and password:

```sh
domain_name=$(aws cloudfront list-distributions --query "DistributionList.Items[?contains(Origins.Items[0].Id, 'http-origin')].DomainName | [0]" --output text)
echo "ArgoCD URL: https://$domain_name/argocd
   Login: admin
   Password: $IDE_PASSWORD"
```

> Wait until Argo CD Load Balancer will be provisioned and UI will be available. It should take about 3-5 minutes.

6. Create secret with Gitea token for Argo CD to access the GitOps repository:

```sh
envsubst << 'EOF' | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
   name: git-${WORKING_REPO}
   namespace: argocd
   labels:
      argocd.argoproj.io/secret-type: repository
stringData:
   url: ${GITEA_EXTERNAL_URL}${GITEA_USERNAME}/${WORKING_REPO}.git
   type: git
   password: $IDE_PASSWORD
   username: $GITEA_USERNAME
EOF
```

7. Login to Argo CD UI and Refresh `bootstrap` Argo CD application.

> Wait until all Argo CD applications will be deployed successfully.

```sh
kubectl get applications -n argocd
```

```sh
NAME                            SYNC STATUS   HEALTH STATUS
ack-ec2-hub-cluster             Synced        Healthy
ack-eks-hub-cluster             Synced        Healthy
...
multi-acct-hub-cluster          Synced        Healthy
```

### Bootstrapping Management/Spoke accounts

In order for the management cluster to execute actions in the spoke AWS accounts, we need to create IAM roles in the spoke accounts:

- `eks-cluster-mgmt-ec2`
- `eks-cluster-mgmt-eks`
- `eks-cluster-mgmt-iam`

1. Execute the script to configure IAM roles in the account. It is **required** for Management account:

```sh
cd $WORKSPACE_PATH/$WORKING_REPO/scripts
./create_ack_workload_roles.sh
```

> Optional. Repeat this step for each of your Spoke accounts you want to use with the solution.
If you want to add spoke accounts, you need to connect to your AWS spoke account. For example, by switching the profile.

```sh
# export AWS_PROFILE=spoke_account1 # use your own profile or be sure to be connected to the appropriate account
```

### Checking kro resources

1. Restart kro to take into account new ACK CRD deployed by ArgoCD

```sh
kubectl rollout restart deployment -n kro-system kro
```

2. Check that the Resource Graph Definitions (RGD) are properly reconciled and active:

```sh
kubectl get resourcegraphdefinitions.kro.run
```

Output expected:

```sh
NAME                        APIVERSION   KIND                STATE    AGE
ekscluster.kro.run          v1alpha1     EksCluster          Active   13m
eksclusterwithvpc.kro.run   v1alpha1     EksclusterWithVpc   Active   12m
vpc.kro.run                 v1alpha1     Vpc                 Active   13m
```

3. If some of the RGD are not active, restart kro again and check:

```sh
kubectl rollout restart deployment -n kro-system kro
```

```sh
kubectl get resourcegraphdefinitions.kro.run
```

### Creating a Spoke cluster(s).

1. Configure spoke cluster accounts in Argo CD application for ACK controller:

ACK controllers can work cross AWS accounts, but that need to isolate resources into specific namespaces. We are doing this with this solution, where each ACK resources will be isolated into specific namespaces.

If you want to do deploy your EKS clusters in multiple AWS accounts, you need to update this configuration.
If you just want to only use one account, you still need to specify the AWS account to use for each namespaces.

We use management account ID to deploy `test`, `pre-prod`, `prod-eu` and `prod-us` clusters:

```sh
sed -i 's/MANAGEMENT_ACCOUNT_ID/'"$ACCOUNT_ID"'/g' "$WORKSPACE_PATH/$WORKING_REPO/addons/tenants/tenant1/default/addons/multi-acct/values.yaml"
code $WORKSPACE_PATH/$WORKING_REPO/addons/tenants/tenant1/default/addons/multi-acct/values.yaml
```

> Uncomment cluster-* in the file.

2. Add, Commit and Push

```sh
cd $WORKSPACE_PATH/$WORKING_REPO/
git status
git add .
git commit -m "add namespaces and resources for clusters"
git push
```

3. Refresh `multi-acct-hub-cluster` Argo CD application to create namespaces for clusters.

4. Update cluster definitions with Management account ID:

```sh
sed -i 's/MANAGEMENT_ACCOUNT_ID/'"$ACCOUNT_ID"'/g' "$WORKSPACE_PATH/$WORKING_REPO/fleet/kro-values/tenants/tenant1/kro-clusters/values.yaml"
code $WORKSPACE_PATH/$WORKING_REPO/fleet/kro-values/tenants/tenant1/kro-clusters/values.yaml
```
> Uncomment cluster-* in the file.
> The clusters will be deployed to `eu-central-1`, `eu-west-1` and `us-west-2`.

5. Add, Commit and Push

```sh
cd $WORKSPACE_PATH/$WORKING_REPO/
git status
git add .
git commit -m "add clusters definitions"
git push
```

6. Refresh `clusters` Argo CD application to start deployment of the clusters.

7. After some times, the clusters should have been created in the spoke/management account(s):

```sh
kubectl get EksClusterwithvpcs -A
```

```sh
NAMESPACE   NAME                STATE    SYNCED   AGE
argocd      cluster-pre-prod   IN_PROGRESS   False    110s
argocd      cluster-prod-eu    IN_PROGRESS   False    110s
argocd      cluster-prod-us    IN_PROGRESS   False    110s
argocd      cluster-test       IN_PROGRESS   False    110s
```

```sh
NAMESPACE   NAME                STATE    SYNCED   AGE
argocd      cluster-pre-prod   ACTIVE   True     31m
argocd      cluster-test       ACTIVE   True     51m
```

> If You see STATE=ERROR, that's may be normal as it will take some times for all dependencies to be OK, but you may want to see the logs of kro and ACK controllers in case you may have some configuration errors.

You can also list resources created by kro to validate their status:

```sh
kubectl get vpcs.kro.run -A
kubectl get vpcs.ec2.services.k8s.aws -A -o yaml # check there is not error
kubectl get vpcs -A
```

> If you see errors, you may need to double check the multi-cluster accounts settings, and if IAM roles in both management and spoke AWS accounts are properly configured.

8. When VPC are ok, then check for EKS resources:

```sh
kubectl get eksclusters.kro.run -A
kubectl get clusters.eks.services.k8s.aws -A -o yaml # Check there are no errors
kubectl get eksclusters -A
kubectl get eksclusterwithvpc -A
```

Check that all ArgoCD Applications have been deployed to spoke clusters:

```sh
kubectl get applications -n argocd
```

```sh
NAME                                 SYNC STATUS   HEALTH STATUS
ack-ec2-hub-cluster                  Synced        Healthy
ack-eks-hub-cluster                  Synced        Healthy
ack-iam-hub-cluster                  Synced        Healthy
argo-rollouts-cluster-pre-prod       Synced        Healthy
argo-rollouts-cluster-prod-eu        Synced        Healthy
argo-rollouts-cluster-prod-us        Synced        Healthy
argo-rollouts-cluster-test           Synced        Healthy
...
metrics-server-cluster-pre-prod      Synced        Healthy
metrics-server-cluster-prod-eu       Synced        Healthy
metrics-server-cluster-prod-us       Synced        Healthy
metrics-server-cluster-test          Synced        Healthy
metrics-server-hub-cluster           Synced        Healthy
multi-acct-hub-cluster               Synced        Healthy
```

### Creating a container image for demo application.

We will use [Argo Rollouts Demo](https://github.com/argoproj/rollouts-demo) application as an example of a workload.

1. Create Amazon Elastic Container Repository (Amazon ECR) for container images of the application:

```sh
aws ecr create-repository --repository-name rollouts-demo --region $AWS_REGION
```

2. Clone the application sources repository and build an initial container image:

```sh
cd $WORKSPACE_PATH
git clone https://github.com/argoproj/rollouts-demo.git
$WORKSPACE_PATH/kro/examples/aws/eks-cluster-mgmt/scripts/build-rollouts-demo.sh
```

3. We already deployed Kargo to the Hub cluster. Create IAM role for Kargo to access Amazon ECR:

```sh
cat << EOF > trust-policy.json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "pods.eks.amazonaws.com"
      },
      "Action": [
        "sts:AssumeRole",
        "sts:TagSession"
      ]
    }
  ]
}
EOF

aws iam create-role --no-cli-pager \
  --role-name kargo-controller-role \
  --assume-role-policy-document file://trust-policy.json --
aws iam attach-role-policy --no-cli-pager \
  --role-name kargo-controller-role \
  --policy-arn arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPowerUser
aws eks create-pod-identity-association \
  --cluster-name hub-cluster \
  --role-arn arn:aws:iam::$ACCOUNT_ID:role/kargo-controller-role \
  --namespace kargo \
  --service-account kargo-controller
rm trust-policy.json
```

### Onboarding a workload example application with continuous promotion.

1. Create Git repository for the application deployment configuration:

```sh
curl -X 'POST' \
   "http://$GITEA_USERNAME:$IDE_PASSWORD@localhost:9000/api/v1/user/repos" \
   -H 'accept: application/json' \
   -H 'Content-Type: application/json' \
   -d "{
      \"name\": \"rollouts-demo-deploy\"
   }"
```

2. Clone and populate the deployment Git repository:

```sh
git clone ssh://git@$GIT_SSH_ENDPOINT/$GITEA_USERNAME/rollouts-demo-deploy.git $WORKSPACE_PATH/rollouts-demo-deploy
cp -r $WORKSPACE_PATH/$WORKING_REPO/apps/rollouts-demo-deploy/* $WORKSPACE_PATH/rollouts-demo-deploy/
```

4. Update Account ID in ECR Urls and repoUrl in the application configuration for Kargo:

```sh
find "$WORKSPACE_PATH/rollouts-demo-deploy" -type f -exec sed -i'' -e "s|GIT_EXAMPLES_URL|${GITEA_EXTERNAL_URL}${GITEA_USERNAME}|g" {} +
find "$WORKSPACE_PATH/rollouts-demo-deploy" -type f -exec sed -i'' -e "s|MANAGEMENT_ACCOUNT_ID|${ACCOUNT_ID}|g" {} +
find "$WORKSPACE_PATH/rollouts-demo-deploy" -type f -exec sed -i'' -e "s|AWS_REGION|${AWS_REGION}|g" {} +

find "$WORKSPACE_PATH/$WORKING_REPO/workloads" -type f -exec sed -i'' -e "s|GIT_EXAMPLES_URL|${GITEA_EXTERNAL_URL}${GITEA_USERNAME}|g" {} +
```

5. Enable `workloads` in Argo CD application:

```sh
cat << EOF > $WORKSPACE_PATH/$WORKING_REPO/workloads/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - rollouts-demo-application-set.yaml
  - rollouts-demo-kargo.yaml
EOF
```

6. Give access Argo CD to Git `rollouts-demo-deploy` repository:

```sh
envsubst << 'EOF' | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
   name: git-rollouts-demo-deploy
   namespace: argocd
   labels:
      argocd.argoproj.io/secret-type: repository
stringData:
   url: ${GITEA_EXTERNAL_URL}${GITEA_USERNAME}/rollouts-demo-deploy.git
   type: git
   password: $IDE_PASSWORD
   username: $GITEA_USERNAME
EOF
```

6. Add, Commit and Push:

```sh
cd $WORKSPACE_PATH/rollouts-demo-deploy/
git status
git add .
git commit -m "Initial commit"
git push

cd $WORKSPACE_PATH/$WORKING_REPO/
git status
git add .
git commit -m "Enable rollouts-demo-deploy"
git push
```

7. Login to Argo CD UI and Refresh `cluster-workloads` application. Wait until `rollouts-demo-kargo` Argo CD application synced and ready.

### Accessing Karo and working with promotions.

1. Access Kargo UI:

```sh
export KARGO_URL=http://$(kubectl get svc kargo-api -n kargo -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
curl --head -X GET --retry 20 --retry-all-errors --retry-delay 15 \
  --connect-timeout 5 --max-time 10 -k $KARGO_URL
echo "Kargo url: $KARGO_URL"
echo "Kargo password: kargo-password"
```

2. Kargo project `rollouts-demo-kargo ` should be available, but `rollouts-demo` Warehouse cannot access Git repository. Give access Kargo to Git `rollouts-demo-deploy` repository:

```sh
envsubst << 'EOF' | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
   name: git-rollouts-demo-kargo
   namespace: rollouts-demo-kargo
   labels:
      kargo.akuity.io/cred-type: git
stringData:
   repoURL:  ${GITEA_EXTERNAL_URL}${GITEA_USERNAME}/rollouts-demo-deploy.git
   password: $IDE_PASSWORD
   username: $GITEA_USERNAME
EOF

kubectl -n kargo rollout restart deploy kargo-controller
```

3. Wait until `rollouts-demo-*` ArgoCD Applications are fully operational and test them.

4. Build a new container images and observe deployment, continuous promotion from `test` to `prep-prod` and rollouts:

```sh
$WORKSPACE_PATH/kro/examples/aws/eks-cluster-mgmt/scripts/build-rollouts-demo.sh orange
```

### Promote the application to prod clusters

1. Login to Kargo UI and `Promote` active Freight to `prod-eu` and `prod-us`.

2. Wait until `rollouts-demo-prod-eu` and `rollouts-demo-prod-us` are fully operational and test Urls.

3. Build a new container image and test complete continuous promotion process:

```sh
$WORKSPACE_PATH/kro/examples/aws/eks-cluster-mgmt/scripts/build-rollouts-demo.sh green
```

<!-- ```sh
envsubst << 'EOF' | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
   name: git-workloads-infra-kargo
   namespace: workloads-infra-kargo
   labels:
      kargo.akuity.io/cred-type: git
stringData:
   repoURL: ${GITEA_EXTERNAL_URL}${GITEA_USERNAME}/${WORKING_REPO}.git
   password: $IDE_PASSWORD
   username: $GITEA_USERNAME
EOF
kubectl -n kargo rollout restart deploy kargo-controller
``` -->

![Kargo continuous promotion](docs/kargo-promotion.png)

## Conclusion

The solution provides a capability to create EKS clusters with kro RGD, deployed to AWS using ACK controllers, and then automatically registered to Argo CD. Argo CD installs addons and workloads automatically. Kargo integrates with Argo CD and Argo Rollouts and enables continuous promotion of workloads to various environment using Stages and Promotion Tasks.

## Cleanup

As our spoke workloads have been bootstrapped by Argo CD and kro/ACK, we need to clean them in specific order.

### Clean up workloads.

1. Disable `workloads` Argo CD application:

```sh
mv $WORKSPACE_PATH/$WORKING_REPO/fleet/bootstrap/workloads.yaml $WORKSPACE_PATH/$WORKING_REPO/fleet/bootstrap/excluded/
```

2. Add, Commit and Push:

```sh
cd $WORKSPACE_PATH/$WORKING_REPO/
git status
git add .
git commit -m "Disable Cluster-workloads Argo CD application"
git push
```

3. Login to Argo CD UI and Sync `bootstrap` Argo CD application with `Prune` option checked. Wait until `cluster-workloads` will be deleted.

4. Delete the demo application project namespace:

```sh
kubectl delete ns rollouts-demo-kargo
```

### Clean up spoke clusters

1. Disable workload clusters in the configuration:

```sh
code $WORKSPACE_PATH/$WORKING_REPO/fleet/kro-values/tenants/tenant1/kro-clusters/values.yaml
```

2. Add, Commit and Push

```sh
cd $WORKSPACE_PATH/$WORKING_REPO/
git status
git add .
git commit -m "Comment out workload clusters"
git push
```

3. Login to Argo CD UI and Sync `clusters` Argo CD application with `Prune` option checked.

4. Wait until all `clusters` and `VPCs` will be deleted:

```sh
kubectl get eksclusterwithvpc -A
kubectl get vpcs -A
```

```sh
NAMESPACE   NAME               STATE      SYNCED   AGE
argocd      cluster-pre-prod   DELETING   False    4h34m
argocd      cluster-prod-eu    DELETING   False    114m
argocd      cluster-prod-us    DELETING   False    114m
argocd      cluster-test       DELETING   False    4h54m
```

5. Once you have successfully de-registered all spoke EKS clusters, you can remove the Hub cluster created with Terraform and created roles:

```sh
cd $WORKSPACE_PATH/$WORKING_REPO/terraform/hub
./destroy.sh
```

You may need to delete remaining Load Balancers, Hub cluster and hub VPC manually if Terraform will not be able to clean it up.

```sh
cd $WORKSPACE_PATH/$WORKING_REPO/scripts/
./delete_ack_workload_roles.sh eks-cluster-mgmt-iam eks-cluster-mgmt-ec2 eks-cluster-mgmt-eks
```

6. Delete ECR repository:

```sh
aws ecr delete-repository --repository-name rollouts-demo --force
```

7. Delete `ide-stack` in AWS CloudFormation.

8. Clean up all the remained resources manually.

Open questions:

- Autogenerated names for addons in the cluster can follow another pattern:
instead of ack-efs-cluster-test -> cluster-test-ack-efs = $cluster_name-$addon_name/workload_name
It will group addons from the same cluster together in Argo CD UI.

- KRO resources are not active after deployment of the Hub cluster with addons and require restart twice. Why?

```sh
kubectl get resourcegraphdefinitions.kro.run
kubectl rollout restart deployment -n kro-system kro
kubectl get resourcegraphdefinitions.kro.run
kubectl rollout restart deployment -n kro-system kro
kubectl get resourcegraphdefinitions.kro.run
```

## Available applications in the hub cluster, links and usernames/passwords.

> The same password is used for all applications excepr Kargo.

```sh
domain_name=$(aws cloudfront list-distributions --query "DistributionList.Items[?contains(Origins.Items[0].Id, 'http-origin')].DomainName | [0]" --output text)
echo "ArgoCD URL: https://$domain_name/argocd
   Login: admin
   Password: $IDE_PASSWORD
   or using Keycloak SSO. Login user1, password $IDE_PASSWORD"
   

echo "Backstage: https://$domain_name
   SSO Login: user1
   Password: $IDE_PASSWORD"

echo "Argo-Workflows: https://$domain_name/argo-workflows
   SSO Login: user1
   Password: $IDE_PASSWORD"

echo "Gitlab: https://$domain_name/gitlab
   Login: root
   Password: $IDE_PASSWORD"

echo Gitea:
env | grep GITEA

export KARGO_URL=http://$(kubectl get svc kargo-api -n kargo -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
echo "Kargo url: $KARGO_URL"
echo "Kargo password: kargo-password"
```
