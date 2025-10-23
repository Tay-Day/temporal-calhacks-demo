# Running

1. Download [go](https://go.dev/dl/)
- Follow instructions on go website or use package manager
- Mac
    ```shell
    brew install go
    ```
- Windows
    ```
    scoop install go
    ```
2. Download [Temporal](https://learn.temporal.io/getting_started/go/dev_environment)
- Follow instructions in temporal docs or use package manager
- Mac
    ```shell
    brew install temporal
    ```
- Windows
    ```shell
    scoop install temporal
    ```
3. In one shell start temporal on 7233 (see port config in main.go)
    ```shell
    temporal server start-dev
    ```
4. In another shell start the backend temporal worker
    ```shell
    cd backend
    go run .
    ```
