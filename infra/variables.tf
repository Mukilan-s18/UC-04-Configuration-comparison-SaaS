variable "environment" {
  type        = string
  description = "Deployment environment name (dev or test)"
  default     = "dev"
}

variable "resource_group_name" {
  type        = string
  description = "Pre-provisioned Azure Resource Group name"
  default     = "rg-advantix-sandbox"
}

variable "location" {
  type        = string
  description = "Azure region for resources"
  default     = "eastus"
}

variable "container_apps_environment_id" {
  type        = string
  description = "ID of the pre-provisioned Container App Environment"
  default     = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-advantix-sandbox/providers/Microsoft.App/managedEnvironments/cae-advantix"
}

variable "storage_account_name" {
  type        = string
  description = "Pre-provisioned Storage Account name"
  default     = "stadvantixsandbox"
}

variable "container_image" {
  type        = string
  description = "Container image for the SaaS service"
  default     = "mcr.microsoft.com/azuredocs/aci-helloworld:latest"
}

variable "cpu" {
  type        = number
  description = "CPU allocation for the container app"
  default     = 0.5
}

variable "memory" {
  type        = string
  description = "Memory allocation for the container app"
  default     = "1Gi"
}
