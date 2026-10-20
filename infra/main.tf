# 1. Private Blob Container Resource
resource "azurerm_storage_container" "evidence_blob" {
  name                  = "uc04-evidence-${var.environment}"
  storage_account_name  = var.storage_account_name
  container_access_type = "private"
}

# 2. Azure Container App Resource
resource "azurerm_container_app" "saas_app" {
  name                         = "ca-configguard-${var.environment}"
  container_app_environment_id = var.container_apps_environment_id
  resource_group_name          = var.resource_group_name
  revision_mode                = "Single"
  workload_profile_name        = "Consumption"

  lifecycle {
    ignore_changes = [
      template[0].http_scale_rule,
      ingress[0].transport,
      ingress[0].traffic_weight,
      template[0].container[0].liveness_probe[0].transport
    ]
  }

  template {
    container {
      name   = "configguard"
      image  = var.container_image
      cpu    = var.cpu
      memory = var.memory

      env {
        name  = "PORT"
        value = "8080"
      }
      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }
      env {
        name  = "BLOB_CONTAINER_NAME"
        value = azurerm_storage_container.evidence_blob.name
      }

      liveness_probe {
        port                    = 8080
        transport               = "HTTP"
        path                    = "/health"
        initial_delay           = 5
        interval_seconds        = 10
        timeout                 = 3
        failure_count_threshold = 3
      }
    }
  }

  ingress {
    allow_insecure_connections = false
    external_enabled           = true
    target_port                = 8080
    traffic_weight {
      percentage      = 100
      latest_revision = true
    }
  }

  tags = {
    Project     = "Advantix-SaaS-Engineering"
    UseCase     = "UC-04"
    Environment = var.environment
  }
}
