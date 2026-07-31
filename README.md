# KuroHelper

## Build

Requires Go 1.26 or later. Clone `kurohelper` and `kurohelper-service` into the same directory, then create a Go workspace:

```powershell
git clone https://github.com/kuro-helper/kurohelper.git
git clone https://github.com/kuro-helper/kurohelper-service.git

go work init ./kurohelper ./kurohelper-service

Set-Location ./kurohelper
go mod download
go build -o kurohelper ./cmd
```

Copy `.env.example` to `.env` before running the built executable.
