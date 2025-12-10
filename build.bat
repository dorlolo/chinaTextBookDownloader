@echo off
echo Building downloader for Windows...
go build -o downloader.exe .
if %errorlevel% == 0 (
    echo Build successful!
    echo Output: downloader.exe
) else (
    echo Build failed with error level %errorlevel%
    exit /b %errorlevel%
)