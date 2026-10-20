output "container_app_fqdn" {
  description = "Public FQDN of the deployed Container App"
  value       = azurerm_container_app.saas_app.latest_revision_fqdn
}

output "health_endpoint" {
  description = "Public URL for /health check"
  value       = "https://${azurerm_container_app.saas_app.latest_revision_fqdn}/health"
}

output "blob_container_name" {
  description = "Name of the private evidence blob container"
  value       = azurerm_storage_container.evidence_blob.name
}
