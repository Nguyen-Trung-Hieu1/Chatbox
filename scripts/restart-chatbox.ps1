$ErrorActionPreference = 'Stop'
$chatboxUrl = 'http://localhost:8088'

try {
    $response = Invoke-WebRequest -Uri $chatboxUrl -TimeoutSec 5
    if ($response.StatusCode -eq 200) {
        Write-Host 'Chatbox1 is already running at http://localhost:8088'
        exit 0
    }
} catch {
    # Continue with a clean WSL restart.
}

Stop-ScheduledTask -TaskName 'Chatbox1 WSL Startup' -ErrorAction SilentlyContinue
wsl.exe --shutdown
Start-Sleep -Seconds 2
Start-ScheduledTask -TaskName 'Chatbox1 WSL Startup'

for ($attempt = 1; $attempt -le 45; $attempt++) {
    Start-Sleep -Seconds 2
    try {
        $response = Invoke-WebRequest -Uri $chatboxUrl -TimeoutSec 5
        if ($response.StatusCode -eq 200) {
            Write-Host 'Chatbox1 is ready at http://localhost:8088'
            exit 0
        }
    } catch {
        # K3s and the application can need up to 90 seconds after WSL starts.
    }
}

throw 'Chatbox1 did not become ready within 90 seconds.'
