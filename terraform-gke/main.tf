
resource "google_container_cluster" "gke_cluster" {
  name     = var.cluster_name
  location = var.zone

  # Create the cluster without a default node pool
  initial_node_count = 0

  deletion_protection = false
}

resource "google_container_node_pool" "primary_nodes" {
  name     = "dubai-properties-node-pool"
  location = var.zone
  cluster  = google_container_cluster.gke_cluster.name

  # Exactly 3 worker nodes
  node_count = 3

  node_config {
    machine_type = var.machine_type
    disk_size_gb = 30
    disk_type    = "pd-standard"

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
  }
}

