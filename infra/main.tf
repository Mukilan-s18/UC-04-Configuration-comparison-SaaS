# 1. Private Blob Container Resource
resource "azurerm_storage_container" "evidence_blob" {
  name                  = "uc04-evidence-${var.environment}"
  storage_account_name  = var.storage_account_name
  container_access_type = "private"
}

