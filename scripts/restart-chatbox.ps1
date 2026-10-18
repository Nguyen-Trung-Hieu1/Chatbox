$ErrorActionPreference = 'Stop'
$chatboxUrl = 'http://localhost:8088'
$argoCdUrl = 'https://localhost:8089'

function Test-LocalUrl {
    param(
        [string]$Url,
        [switch]$SkipCertificateCheck
    )

    try {
        if ($SkipCertificateCheck) {
            & curl.exe -k -s -o NUL -w '%{http_code}' --max-time 5 $Url | ForEach-Object {
                return $_ -eq '200'
            }
        }

        $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 5
        return $response.StatusCode -eq 200
    } catch {
        return $false
    }
}

if ((Test-LocalUrl -Url $chatboxUrl) -and
    (Test-LocalUrl -Url $argoCdUrl -SkipCertificateCheck)) {
    Write-Host 'Chatbox1 and Argo CD are already running.'
    exit 0
}

Stop-ScheduledTask -TaskName 'Chatbox1 WSL Startup' -ErrorAction SilentlyContinue
wsl.exe --shutdown
Start-Sleep -Seconds 2
Start-ScheduledTask -TaskName 'Chatbox1 WSL Startup'

for ($attempt = 1; $attempt -le 45; $attempt++) {
    Start-Sleep -Seconds 2
    if ((Test-LocalUrl -Url $chatboxUrl) -and
        (Test-LocalUrl -Url $argoCdUrl -SkipCertificateCheck)) {
        Write-Host 'Chatbox1 is ready at http://localhost:8088'
        Write-Host 'Argo CD is ready at https://localhost:8089'
        exit 0
    }
}

throw 'Chatbox1 or Argo CD did not become ready within 90 seconds.'
