# Déploie les fonctions d'exemple via l'API Admin Sovrabase.
# Usage:
#   $env:SOVRABASE_URL = "http://localhost:6070"
#   $env:SOVRABASE_ADMIN_TOKEN = "<jwt-admin>"
#   $env:SOVRABASE_PROJECT_ID = "<project-id>"
#   .\deploy.ps1

$base = $env:SOVRABASE_URL ?? "http://localhost:6070"
$token = $env:SOVRABASE_ADMIN_TOKEN
$projectId = $env:SOVRABASE_PROJECT_ID

if (-not $token -or -not $projectId) {
  Write-Error "Définissez SOVRABASE_ADMIN_TOKEN et SOVRABASE_PROJECT_ID"
  exit 1
}

$headers = @{
  "Authorization" = "Bearer $token"
  "Content-Type"  = "application/json"
}

$functions = @(
  @{
    name    = "hello"
    trigger = "http"
    public  = $true
    code    = (Get-Content -Raw "$PSScriptRoot\hello-http.js")
  },
  @{
    name       = "on-user-create"
    trigger    = "record:create"
    collection = "users"
    code       = (Get-Content -Raw "$PSScriptRoot\on-user-create.js")
  }
)

foreach ($fn in $functions) {
  $body = $fn | ConvertTo-Json -Depth 5
  $uri = "$base/admin/projects/$projectId/functions"
  Write-Host "Déploiement: $($fn.name)..."
  Invoke-RestMethod -Method POST -Uri $uri -Headers $headers -Body $body
}

Write-Host "Terminé. Test HTTP: GET $base/api/v1/functions/hello?project_key=<CLE>&name=Sovrabase"
