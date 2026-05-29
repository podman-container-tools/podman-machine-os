# Output info so we know what version we are testing.
wsl --version

Write-Host "Updating PATH environment variable to include Podman's bin directory`n"
$env:Path = $env:Path + ";" + [System.Environment]::GetEnvironmentVariable("Path", "Machine") +
            ";" +
            [System.Environment]::GetEnvironmentVariable("Path", "User")

Write-Host "Podman version"
podman.exe --version

$Env:CONTAINERS_MACHINE_PROVIDER = "${ENV:PROVIDER}"
$Env:MACHINE_IMAGE_PATH="..\${ENV:MACHINE_IMAGE}"
.\bin\ginkgo -v .\verify
if ( ($LASTEXITCODE -ne $null) -and ($LASTEXITCODE -ne 0) ) {
    throw "Exit code = '$LASTEXITCODE' running ginkgo"
}
