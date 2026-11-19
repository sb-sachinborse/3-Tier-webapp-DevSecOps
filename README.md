# Dubai Properties --- 3-Tier DevSecOps on Google Kubernetes Engine

A hands-on DevOps portfolio project that packages a real-estate
application into containers, validates the code with a Jenkins CI
pipeline, publishes versioned images to Docker Hub, and deploys the
application to Google Kubernetes Engine (GKE).

> **Portfolio scope:** This project demonstrates an end-to-end CI/CD
> workflow using Jenkins, Docker, GCP, GKE, Terraform, and security
> scanning. It is a learning/portfolio deployment, not a claim of
> production certification or production-grade hardening.

------------------------------------------------------------------------

## 1. Project overview

The application is a three-tier real-estate web application:

  -----------------------------------------------------------------------
  Tier                    Technology              Responsibility
  ----------------------- ----------------------- -----------------------
  Presentation            React 18 + Vite + NGINX User interface, static
                                                  asset serving, API
                                                  reverse proxy

  Application             Go (`net/http`)         REST API,
                                                  authentication,
                                                  property operations,
                                                  authorization

  Data                    MongoDB 7               Application data and
                                                  GridFS image storage
  -----------------------------------------------------------------------

Application capabilities include user registration/login, JWT-based
authentication, property browsing/search/filtering, property CRUD
operations, role-based owner/admin authorization, and image uploads
stored using MongoDB GridFS.

### Repository and image locations

-   **Application repository:**
    `https://github.com/sb-sachinborse/3-Tier-webapp-DevSecOps`
-   **Branch used by Jenkins:** `dev`
-   **Backend image:** `mavrickdocker/dubai-properties-backend`
-   **Frontend image:** `mavrickdocker/dubai-properties-frontend`
-   **Docker Hub account:** `mavrickdocker`

The Jenkins pipeline was configured in the Jenkins job's Pipeline Script
UI during this project. The job checked out the application repository's
`dev` branch; the pipeline script itself was maintained in Jenkins
rather than being read from a repository `Jenkinsfile`.

------------------------------------------------------------------------

## 2. Architecture

``` mermaid
flowchart TB
    U[User browser]
    subgraph GCP[Google Cloud Platform]
        subgraph GKE[ GKE cluster: dubai-properties-gke ]
            subgraph NS[Namespace: dubai-properties]
                FE[Frontend Deployment\nReact build served by NGINX\nService: frontend :80]
                BE[Backend Deployment\nGo REST API\nService: backend :8081]
                DB[MongoDB Deployment\nMongoDB 7]
                PVC[(PersistentVolumeClaim\nmongo-pvc)]
                SEC[Kubernetes Secrets\nbackend-secret / mongo-secret]
                FE -->|/api reverse proxy| BE
                BE -->|MongoDB connection| DB
                DB --- PVC
                BE -. reads credentials .-> SEC
                DB -. reads credentials .-> SEC
            end
        end
    end
    U -->|port-forward for demo| FE
    subgraph CI[CI/CD on Ubuntu host]
        J[Jenkins job: 3-tier-pipeline]
        GIT[GitHub: branch dev]
        TOOLS[Gitleaks • SonarQube • Trivy]
        DOCKER[Docker build]
        DH[Docker Hub]
        GCLOUD[gcloud + kubectl]
        GIT --> J
        J --> TOOLS
        J --> DOCKER
        DOCKER --> DH
        J --> GCLOUD
    end
    DH -. image pull .-> FE
    DH -. image pull .-> BE
    GCLOUD -->|update deployments / verify rollout| GKE
```

### Request flow

1.  The browser loads the React application from the frontend NGINX
    service.
2.  The frontend's NGINX configuration proxies requests beginning with
    `/api/` to the Kubernetes `backend` Service on port `8081`.
3.  The Go API handles authentication and property endpoints.
4.  The backend connects to the MongoDB service using credentials
    supplied through Kubernetes Secrets.
5.  MongoDB stores application records and uploaded images in GridFS;
    its persistent volume claim provides persistent storage beyond an
    individual MongoDB pod restart.

### CI/CD flow

``` mermaid
flowchart TD
    A[Developer pushes to GitHub dev branch] --> B[Jenkins checkout]
    B --> C[Parallel build: Go backend + React frontend]
    C --> D[Go unit tests]
    D --> E[Gitleaks scan]
    E --> F[SonarQube analysis]
    F --> G[Docker build: backend + frontend]
    G --> H[Trivy image scans]
    H --> I[Push versioned images + latest to Docker Hub]
    I --> J[Jenkins authenticates to GKE]
    J --> K[kubectl set image]
    K --> L[Deployment rollout status]
    L --> M[Print Kubernetes pod status]
```

**Important implementation detail:** Gitleaks and Trivy were configured
as report-only checks in the pipeline (`|| true`), so findings did not
fail the build. The SonarQube Quality Gate stage was a placeholder at
the time the project was completed; a real `waitForQualityGate`
enforcement step remains a possible improvement.

------------------------------------------------------------------------

## 3. Cloud and deployment environment

  Item                          Value
  ----------------------------- ----------------------------------
  Cloud                         Google Cloud Platform
  GCP project                   `project-a70c7f47-e201-489a-87f`
  GKE cluster                   `dubai-properties-gke`
  GKE zone                      `us-central1-a`
  Kubernetes namespace          `dubai-properties`
  Node pool                     3 nodes
  Machine type                  `e2-medium`
  Node disk                     30 GB `pd-standard`
  Kubernetes version observed   `v1.36.4-gke.2377000`
  Jenkins job                   `3-tier-pipeline`
  Jenkins URL (local)           `http://localhost:8080`
  SonarQube endpoint used       `http://13.60.41.68:9000`

The GKE cluster and node pool were provisioned using Terraform in the
`terraform-gke/` directory. The cluster was configured with
`initial_node_count = 0`, and a separate node pool supplied the three
worker nodes. The Terraform configuration used
`deletion_protection = false`.

> Do not commit Terraform state, `.tfvars` containing secrets,
> Kubernetes Secret manifests with real values, Docker Hub credentials,
> JWT secrets, or MongoDB passwords to a public repository.

------------------------------------------------------------------------

## 4. Repository layout

The application repository contains the frontend, backend, Kubernetes
manifests, and Terraform configuration. The exact file list can evolve,
but the main areas are:

``` text
3-Tier-webapp-DevSecOps/
├── backend/
│   ├── Dockerfile
│   ├── go.mod
│   └── Go API source and tests
├── frontend/
│   ├── Dockerfile
│   ├── nginx.conf
│   ├── package.json
│   └── React + Vite source
├── k8s/
│   ├── backend / frontend / MongoDB resources
│   ├── Services, Secrets and storage resources
│   └── jenkins-rbac.yaml
└── terraform-gke/
    ├── main.tf
    ├── provider.tf
    ├── variables.tf
    └── terraform.tfvars
```

Check the actual repository before relying on this illustrative tree in
an interview or changing paths:

``` bash
cd ~/project-jenkins/3-Tier-webapp-DevSecOps
find . -maxdepth 3 -type f | sort
```

------------------------------------------------------------------------

## 5. Application configuration

### Backend

The Go API listens on port `8081`. Its configuration includes:

  Variable           Purpose
  ------------------ ----------------------------------------------------
  `MONGO_URI`        MongoDB connection URI
  `MONGO_DATABASE`   Database name; used as `dubai_properties`
  `JWT_SECRET`       JWT signing secret; must be at least 32 characters
  `PORT`             API listen port; used as `8081`

The backend health endpoint is:

``` http
GET /health
```

Expected response:

``` json
{"status":"UP","service":"dubai-properties-api"}
```

Application endpoints include `/api/auth/register`, `/api/auth/login`,
`/api/auth/me`, `/api/properties`, and `/api/images/{id}`.

### Frontend and NGINX

The frontend uses the Vite build-time variable:

``` javascript
const API = import.meta.env.VITE_API_URL || 'http://localhost:8081/api';
```

The container build supplies `VITE_API_URL=/api`. NGINX serves the built
React application and reverse-proxies `/api/` to `http://backend:8081`.
This means the browser can call the same origin instead of needing to
know the internal Kubernetes Service DNS name.

The frontend health endpoint is:

``` bash
curl http://localhost:8080/health
```

Expected response: `ok`.

### Kubernetes secrets

The namespace contains:

-   `mongo-secret` for MongoDB root credentials.
-   `backend-secret` for backend connection configuration, including
    `MONGO_URI` and `JWT_SECRET`.

Secret values are deliberately not documented here. To confirm that the
Secret objects exist without printing their values:

``` bash
kubectl get secrets -n dubai-properties
```

------------------------------------------------------------------------

## 6. Build and run locally

Run these commands from the application repository root.

### Backend checks

``` bash
cd backend
go mod download
go build ./...
go test ./...
```

### Frontend checks

``` bash
cd ../frontend
npm ci
npm run build
```

### Docker image build

From the repository root:

``` bash
docker build -t dubai-properties-backend:local ./backend
docker build \
  --build-arg VITE_API_URL=/api \
  -t dubai-properties-frontend:local \
  ./frontend
```

The application also had a Docker Compose setup for local testing with
MongoDB, backend, and frontend. Keep local environment values in an
untracked `.env` file; never put real credentials in the README.

------------------------------------------------------------------------

## 7. Terraform: provision GKE

From the application repository root:

``` bash
cd terraform-gke
terraform init
terraform fmt -check
terraform validate
terraform plan
terraform apply
```

Review the plan before applying. Terraform provisions the GKE
infrastructure described by the configuration and state. It does not
automatically manage every Kubernetes object subsequently applied with
`kubectl`.

Useful Terraform commands:

``` bash
terraform workspace show
terraform state list
terraform plan
```

After the cluster is provisioned, configure `gcloud` and retrieve
cluster credentials:

``` bash
gcloud config set project project-a70c7f47-e201-489a-87f

gcloud container clusters get-credentials dubai-properties-gke \
  --zone us-central1-a \
  --project project-a70c7f47-e201-489a-87f

kubectl cluster-info
kubectl get nodes -o wide
```

Create or verify the application namespace:

``` bash
kubectl create namespace dubai-properties \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl get namespaces
```

------------------------------------------------------------------------

## 8. Kubernetes deployment

The application runs in the `dubai-properties` namespace.

### Workload design

  Kubernetes object                   Purpose
  ----------------------------------- --------------------------------------
  `Deployment/backend`                Runs the Go API container
  `Service/backend`                   Internal API endpoint on port `8081`
  `Deployment/frontend`               Runs the NGINX frontend container
  `Service/frontend`                  Frontend endpoint on port `80`
  `Deployment/mongodb`                Runs MongoDB 7
  `Service/mongo`                     Internal MongoDB service
  `PersistentVolumeClaim/mongo-pvc`   Persistent MongoDB data storage
  `Secret/backend-secret`             Backend sensitive configuration
  `Secret/mongo-secret`               MongoDB credentials

The backend and frontend deployments use container image tags published
by Jenkins. The MongoDB PVC was configured as a 10 GiB ReadWriteOnce
claim.

### Inspect resources

``` bash
kubectl get all -n dubai-properties
kubectl get pvc -n dubai-properties
kubectl get secrets -n dubai-properties
kubectl get pods -n dubai-properties -o wide
```

### View deployment configuration and events

``` bash
kubectl describe deployment backend -n dubai-properties
kubectl describe deployment frontend -n dubai-properties
kubectl describe pod -n dubai-properties
kubectl get events -n dubai-properties --sort-by=.metadata.creationTimestamp
```

### View logs

``` bash
kubectl logs deployment/backend -n dubai-properties
kubectl logs deployment/frontend -n dubai-properties
kubectl logs deployment/mongodb -n dubai-properties
```

------------------------------------------------------------------------

## 9. Jenkins CI/CD pipeline

### Jenkins configuration used

  Jenkins configuration      Value
  -------------------------- ------------------------------------------
  Job                        `3-tier-pipeline`
  Source repository          `sb-sachinborse/3-Tier-webapp-DevSecOps`
  Branch                     `dev`
  NodeJS tool                `NodeJS`
  SonarQube server name      `sonar`
  SonarScanner tool          `SonarScanner`
  Docker Hub credential ID   `docker-hub`

Jenkins ran on an Ubuntu host (hostname observed: `DESKTOP-40QSETJ`)
rather than on GCP. Docker Hub authentication was stored in Jenkins
Credentials; the password/token was not hard-coded in the pipeline.

### Pipeline stages

1.  **Checkout** --- fetches the `dev` branch from GitHub.
2.  **Parallel build** --- downloads Go modules/builds the backend and
    installs/builds the React frontend.
3.  **Go tests** --- runs `go test ./...`.
4.  **Gitleaks** --- scans repository content for accidentally committed
    secrets.
5.  **SonarQube** --- submits source analysis to the configured
    SonarQube server.
6.  **Docker build** --- creates backend and frontend images.
7.  **Trivy** --- scans the built images for vulnerabilities and related
    findings.
8.  **Docker Hub push** --- publishes both `$BUILD_NUMBER` and `latest`
    tags.
9.  **GKE authentication** --- obtains cluster credentials and checks
    deployment permissions.
10. **Deploy** --- updates the backend and frontend Deployment images.
11. **Rollout verification** --- waits for both deployments and prints
    pod status.

### Image naming and tagging

For Jenkins build number `N`, the images are:

``` text
mavrickdocker/dubai-properties-backend:N
mavrickdocker/dubai-properties-frontend:N
mavrickdocker/dubai-properties-backend:latest
mavrickdocker/dubai-properties-frontend:latest
```

Using the Jenkins build number gives each pipeline run a distinct image
tag and makes it possible to identify which build is deployed.

### GKE deployment commands used by the pipeline

The key deployment commands are:

``` bash
kubectl set image deployment/backend \
  backend="$DOCKERHUB_USER/$BACKEND_IMAGE:$BUILD_NUMBER" \
  -n "$K8S_NAMESPACE"

kubectl set image deployment/frontend \
  frontend="$DOCKERHUB_USER/$FRONTEND_IMAGE:$BUILD_NUMBER" \
  -n "$K8S_NAMESPACE"
```

Then Jenkins waits for both rollouts:

``` bash
kubectl rollout status deployment/backend \
  -n "$K8S_NAMESPACE" --timeout=180s

kubectl rollout status deployment/frontend \
  -n "$K8S_NAMESPACE" --timeout=180s

kubectl get pods -n "$K8S_NAMESPACE" -o wide
```

### Jenkins-to-GCP authentication

Service-account key creation was blocked by the GCP organization policy.
Instead of creating and storing a JSON service-account key, the setup
used `gcloud` user authentication with service-account impersonation for
the Linux `jenkins` account.

The deployer service account was:

``` text
jenkins-gke-deployer@project-a70c7f47-e201-489a-87f.iam.gserviceaccount.com
```

The Jenkins Linux user's gcloud configuration was set to impersonate
that service account. The service account had the cluster-viewer project
role, and the authenticated user had permission to mint short-lived
impersonated credentials. Kubernetes namespace-scoped RBAC then allowed
the deployment operations.

The relevant configuration commands used during setup were:

``` bash
sudo -u jenkins -H gcloud auth login --no-launch-browser

sudo -u jenkins -H gcloud config set project \
  project-a70c7f47-e201-489a-87f

sudo -u jenkins -H gcloud config set auth/impersonate_service_account \
  jenkins-gke-deployer@project-a70c7f47-e201-489a-87f.iam.gserviceaccount.com
```

The Jenkins user's kubeconfig was generated with:

``` bash
sudo -u jenkins -H gcloud container clusters get-credentials \
  dubai-properties-gke \
  --zone us-central1-a \
  --project project-a70c7f47-e201-489a-87f
```

A token check was performed without printing the token:

``` bash
sudo -u jenkins -H bash -c \
  'gcloud auth print-access-token >/dev/null && echo "GCP authentication successful"'
```

### Kubernetes RBAC for Jenkins

A namespace-scoped Role and RoleBinding were applied using
`k8s/jenkins-rbac.yaml`. The permissions were limited to the resources
needed by the deployment workflow, including:

-   Deployments: `get`, `list`, `watch`, `patch`, `update`
-   ReplicaSets: `get`, `list`, `watch`
-   Pods: `get`, `list`, `watch`
-   Services: `get`, `list`

The intent was to avoid granting Jenkins broad cluster-admin
permissions.

Check permissions with:

``` bash
kubectl auth can-i patch deployments -n dubai-properties
kubectl auth can-i get pods -n dubai-properties
```

Expected output for both checks: `yes`.

------------------------------------------------------------------------

## 10. Access the application

The frontend Service was configured as `ClusterIP`, so it was not
exposed directly to the public internet. For a local demonstration, use
port-forwarding:

``` bash
kubectl port-forward service/frontend 8080:80 \
  -n dubai-properties
```

Open this URL in a browser:

``` text
http://localhost:8080
```

Keep the port-forward terminal running while using the application.

### Smoke tests

In another terminal:

``` bash
curl http://localhost:8080/health
```

Expected:

``` text
ok
```

Check the API through the frontend reverse proxy:

``` bash
curl http://localhost:8080/api/properties
```

The response depends on the data currently stored in MongoDB.

------------------------------------------------------------------------

## 11. Security controls demonstrated

  -----------------------------------------------------------------------
  Control                 Tool / implementation   What it demonstrates
  ----------------------- ----------------------- -----------------------
  Secret detection        Gitleaks                Detecting credentials
                                                  accidentally committed
                                                  to source control

  Static analysis         SonarQube               Source-code quality and
                                                  static analysis

  Container vulnerability Trivy                   Identifying
  scanning                                        vulnerabilities in
                                                  container images

  Containerization        Docker                  Consistent packaging of
                                                  frontend and backend

  Cloud access            GCP IAM +               Avoiding long-lived
                          service-account         service-account JSON
                          impersonation           keys

  Kubernetes permissions  RBAC                    Restricting Jenkins
                                                  deployment permissions
                                                  to the application
                                                  namespace

  Sensitive runtime       Kubernetes Secrets      Keeping application
  configuration                                   credentials out of
                                                  container images

  Persistent application  PVC                     Separating MongoDB data
  data                                            storage from the
                                                  MongoDB pod lifecycle
  -----------------------------------------------------------------------

### Known limitations and honest status

These are important to explain accurately in an interview:

-   Gitleaks and Trivy were report-only in the completed pipeline
    because their commands used `|| true`. They generated scan output
    but did not block a build on findings.
-   The SonarQube Quality Gate stage was still a placeholder echo, not
    an enforced `waitForQualityGate` check.
-   The frontend was accessed through `kubectl port-forward`; a public
    HTTPS ingress/load balancer was not part of the completed setup.
-   Monitoring/alerting, automated MongoDB backup/restore, and
    autoscaling were not completed.
-   The project used a MongoDB PVC, but a tested backup-and-restore
    procedure was not part of the final demo.
-   The pipeline updated images by build-number tag and verified
    rollout; a demonstrated automated rollback workflow was not
    included.

Calling out these boundaries shows engineering judgment. Do not describe
a report-only scan as a blocking security gate or a ClusterIP service as
a public production endpoint.

------------------------------------------------------------------------

## 12. Troubleshooting notes

### A. Jenkins kubectl uses the wrong cluster/context

If `kubectl` under the Jenkins Linux account points to `localhost:8080`
or returns an unexpected response, generate the kubeconfig for that same
Linux account:

``` bash
sudo -u jenkins -H gcloud container clusters get-credentials \
  dubai-properties-gke \
  --zone us-central1-a \
  --project project-a70c7f47-e201-489a-87f

sudo -u jenkins -H kubectl config current-context
sudo -u jenkins -H kubectl get nodes
```

### B. Check whether a deployment rollout is stuck

``` bash
kubectl rollout status deployment/backend \
  -n dubai-properties --timeout=180s

kubectl describe deployment backend -n dubai-properties
kubectl get pods -n dubai-properties
kubectl get events -n dubai-properties \
  --sort-by=.metadata.creationTimestamp
```

### C. Inspect application logs

``` bash
kubectl logs deployment/backend -n dubai-properties
kubectl logs deployment/frontend -n dubai-properties
kubectl logs deployment/mongodb -n dubai-properties
```

### D. Confirm the image currently deployed

``` bash
kubectl get deployment backend -n dubai-properties \
  -o jsonpath='{.spec.template.spec.containers[0].image}'

echo

kubectl get deployment frontend -n dubai-properties \
  -o jsonpath='{.spec.template.spec.containers[0].image}'

echo
```

### E. Confirm Services and storage

``` bash
kubectl get services -n dubai-properties
kubectl get pvc -n dubai-properties
kubectl get pods -n dubai-properties -o wide
```

------------------------------------------------------------------------

## 13. GCP cost cleanup

The GKE cluster, worker nodes, persistent disks, and any other remaining
cloud resources can incur charges. When the project demonstration is
finished, destroy the Terraform-managed infrastructure from the correct
Terraform directory.

First inspect the state and preview the destruction:

``` bash
cd ~/project-jenkins/3-Tier-webapp-DevSecOps/terraform-gke

terraform workspace show
terraform state list
terraform plan -destroy
```

Review the plan carefully. If it contains the expected resources and you
are ready to remove them:

``` bash
terraform destroy
```

Type `yes` only after reviewing the destroy plan.

**Important:** `terraform destroy` only removes resources tracked by
that Terraform state. Kubernetes-created resources, persistent disks,
reserved IP addresses, or other resources not tracked by Terraform may
remain. After destruction, inspect GCP for leftover resources before
assuming billing has stopped. Deleting the cluster or its persistent
disk can permanently remove the MongoDB data.

Useful checks:

``` bash
gcloud container clusters list \
  --project project-a70c7f47-e201-489a-87f

gcloud compute disks list \
  --project project-a70c7f47-e201-489a-87f

gcloud compute addresses list \
  --project project-a70c7f47-e201-489a-87f

gcloud compute instances list \
  --project project-a70c7f47-e201-489a-87f
```

Terraform destruction does not delete the GitHub repository, Docker Hub
images, local Jenkins installation, or local project files.

------------------------------------------------------------------------

## 14. Interview explanation

### 60--90 second project summary

"I built a three-tier real-estate application using React, Go, and
MongoDB, and deployed it on Google Kubernetes Engine. I containerized
the frontend and backend with Docker and created a Jenkins CI/CD
pipeline that checks out code from GitHub, builds both services, runs Go
tests, performs Gitleaks and SonarQube analysis, builds Docker images,
scans them with Trivy, and pushes versioned images to Docker Hub.

For deployment, Jenkins authenticates to GCP using gcloud
service-account impersonation rather than a long-lived service-account
JSON key. I restricted its Kubernetes permissions with namespace-scoped
RBAC. The pipeline updates the backend and frontend Deployments to the
new build-number image tags, waits for rollout completion, and reports
pod status. The frontend NGINX container proxies API requests to the Go
backend, which connects to MongoDB. I used Terraform to provision the
GKE infrastructure."

### Questions to prepare for

1.  Why did you separate frontend, backend, and database into three
    tiers?
2.  Why use Docker images rather than deploy source code directly?
3.  What is the difference between a Kubernetes Deployment and a
    Service?
4.  Why is the frontend NGINX configuration proxying `/api/` to the
    backend Service?
5.  Why use a build-number image tag instead of only `latest`?
6.  What happens if a new image fails its Kubernetes readiness or
    rollout checks?
7.  What does Gitleaks detect? How is it different from Trivy?
8.  What does SonarQube analyze? What is a Quality Gate?
9.  Why was service-account impersonation used instead of a JSON key?
10. What is the purpose of Kubernetes RBAC, and why scope Jenkins to one
    namespace?
11. What is a PersistentVolumeClaim, and what happens to MongoDB data if
    its disk is deleted?
12. How would you add HTTPS ingress, monitoring, backup/restore, and
    enforced security gates?

### Be precise when discussing security

If asked whether the pipeline blocks every vulnerable image, answer
honestly: "Trivy was integrated and produced scan results, but in this
version it was report-only. Making the vulnerability threshold a
blocking gate is a planned improvement." Use the same honesty for
Gitleaks and the SonarQube Quality Gate placeholder.

------------------------------------------------------------------------

## 15. Skills demonstrated

-   Linux command-line administration
-   Git and GitHub workflow
-   Jenkins declarative CI/CD
-   Docker image build, tagging, and registry publishing
-   Go build and unit testing
-   React/Vite production build
-   Gitleaks, SonarQube, and Trivy integration
-   Google Cloud IAM and GKE
-   Terraform infrastructure provisioning
-   Kubernetes Deployments, Services, Secrets, PVCs, rollout
    verification
-   Kubernetes RBAC and least-privilege thinking
-   CI/CD troubleshooting and operational cost cleanup

------------------------------------------------------------------------

## 16. Final project status

**Core portfolio objective achieved:** a working three-tier application
was built, containerized, scanned, published, deployed to GKE, and
updated through Jenkins with rollout verification.

The next step is not necessarily adding more tools. Preserve the
architecture diagram, successful Jenkins run, SonarQube results, Trivy
output, Kubernetes workload status, and application screenshots.
Practise explaining the design decisions, authentication flow,
deployment process, and known limitations.

