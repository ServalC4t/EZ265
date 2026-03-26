# build/package.ps1 — H.265 一発変換 ビルド＆パッケージスクリプト
# 使い方: powershell -ExecutionPolicy Bypass -File build\package.ps1

param(
    [string]$Version = "1.0.0"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ProjectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$DistDir     = Join-Path $ProjectRoot "dist"
$ExeName     = "h265conv.exe"
$ZipName     = "h265conv_v${Version}_windows_x64.zip"

Write-Host "=== H.265 一発変換 ビルド ===" -ForegroundColor Cyan
Write-Host "Version : $Version"
Write-Host "Project : $ProjectRoot"

# --- Clean ---
if (Test-Path $DistDir) {
    Remove-Item -Recurse -Force $DistDir
}
New-Item -ItemType Directory -Path $DistDir | Out-Null
New-Item -ItemType Directory -Path (Join-Path $DistDir "bin") | Out-Null

# --- Embed manifest via rsrc (if available) or windres ---
$ManifestPath = Join-Path $ProjectRoot "assets\h265conv.manifest"
$SysoPath     = Join-Path $ProjectRoot "h265conv_windows_amd64.syso"

# Try rsrc first
$rsrc = Get-Command rsrc -ErrorAction SilentlyContinue
if ($rsrc) {
    Write-Host "[1/4] Embedding manifest via rsrc..." -ForegroundColor Yellow
    & rsrc -manifest $ManifestPath -o $SysoPath
} else {
    # Try windres
    $windres = Get-Command windres -ErrorAction SilentlyContinue
    if ($windres) {
        Write-Host "[1/4] Embedding manifest via windres..." -ForegroundColor Yellow
        $rcContent = "1 24 `"$($ManifestPath -replace '\\','\\')`""
        $rcPath = Join-Path $ProjectRoot "assets\h265conv.rc"
        Set-Content -Path $rcPath -Value $rcContent
        & windres -i $rcPath -o $SysoPath
        Remove-Item $rcPath -ErrorAction SilentlyContinue
    } else {
        Write-Host "[1/4] SKIP: rsrc/windres not found. Manifest will not be embedded." -ForegroundColor DarkYellow
    }
}

# --- Build ---
Write-Host "[2/4] Building $ExeName ..." -ForegroundColor Yellow
Push-Location $ProjectRoot
try {
    $env:CGO_ENABLED = "1"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"

    go build -ldflags="-H windowsgui -w -s" -trimpath -o (Join-Path $DistDir $ExeName) .
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }
} finally {
    Pop-Location
}

# Remove .syso after build
if (Test-Path $SysoPath) {
    Remove-Item $SysoPath -ErrorAction SilentlyContinue
}

Write-Host "  -> $(Join-Path $DistDir $ExeName)" -ForegroundColor Green

# --- UPX (optional) ---
$upx = Get-Command upx -ErrorAction SilentlyContinue
if ($upx) {
    Write-Host "[3/4] Compressing with UPX..." -ForegroundColor Yellow
    & upx --best --lzma (Join-Path $DistDir $ExeName)
} else {
    Write-Host "[3/4] SKIP: UPX not found. Skipping compression." -ForegroundColor DarkYellow
}

# --- Copy ffmpeg/ffprobe ---
$BinSrc = Join-Path $ProjectRoot "bin"
$BinDst = Join-Path $DistDir "bin"
if (Test-Path (Join-Path $BinSrc "ffmpeg.exe")) {
    Copy-Item (Join-Path $BinSrc "ffmpeg.exe")  -Destination $BinDst
    Copy-Item (Join-Path $BinSrc "ffprobe.exe") -Destination $BinDst
    Write-Host "  -> ffmpeg.exe / ffprobe.exe copied" -ForegroundColor Green
} else {
    Write-Host "  WARNING: bin/ffmpeg.exe not found. Place ffmpeg.exe and ffprobe.exe in bin/ before distributing." -ForegroundColor Red
}

# --- ZIP ---
Write-Host "[4/4] Creating $ZipName ..." -ForegroundColor Yellow
$ZipPath = Join-Path $ProjectRoot $ZipName
if (Test-Path $ZipPath) { Remove-Item $ZipPath }
Compress-Archive -Path (Join-Path $DistDir "*") -DestinationPath $ZipPath -Force
Write-Host "  -> $ZipPath" -ForegroundColor Green

# --- Summary ---
Write-Host ""
Write-Host "=== ビルド完了 ===" -ForegroundColor Cyan
$exeSize = (Get-Item (Join-Path $DistDir $ExeName)).Length / 1MB
Write-Host ("  h265conv.exe : {0:N1} MB" -f $exeSize)
if (Test-Path (Join-Path $BinDst "ffmpeg.exe")) {
    $ffSize = (Get-Item (Join-Path $BinDst "ffmpeg.exe")).Length / 1MB
    $fpSize = (Get-Item (Join-Path $BinDst "ffprobe.exe")).Length / 1MB
    Write-Host ("  ffmpeg.exe   : {0:N1} MB" -f $ffSize)
    Write-Host ("  ffprobe.exe  : {0:N1} MB" -f $fpSize)
}
$zipSize = (Get-Item $ZipPath).Length / 1MB
Write-Host ("  ZIP total    : {0:N1} MB" -f $zipSize)
