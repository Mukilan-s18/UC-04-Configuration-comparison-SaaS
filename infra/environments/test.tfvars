environment                  = "test"
resource_group_name          = "rg-advantix-sandbox-test"
location                     = "eastus"
container_apps_environment_id = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-advantix-sandbox-test/providers/Microsoft.App/managedEnvironments/cae-advantix-test"
storage_account_name         = "stadvantixtest01"
container_image              = "ghcr.io/advantix/configguard:v1.0.0"
cpu                          = 0.5
memory                       = "1Gi"
