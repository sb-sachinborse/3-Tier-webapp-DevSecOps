
variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "us-cental1"
}

variable "zone" {
  description = "GKE cluster zone"
  type        = string
  default     = "us-central1-a"
}

variable "cluster_name" {
  description = "dubai-property-cluster"
  type        = string
  default     = "dubai-properties-gke"
}

variable "machine_type" {
  description = "GKE worker node machine type"
  type        = string
  default     = "e2-medium"
}

