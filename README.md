# Conversion CalHacks Demo - Temporal

...

## Requirements

- Download [go](https://go.dev/dl/). Follow the suggested instructions or use your packaege manager like below

  ```shell
  # macOS
  brew install go

  # Windows
  scoop install go
  ```

- Download [Temporal](https://learn.temporal.io/getting_started/go/dev_environment). Follow the suggested instructions or use your packaege manager like below

  ```shell
  # macOS
  brew install temporal

  # Windows
  scoop install temporal
  ```

- Optionally, download [Node](https://nodejs.org/en/download) and [pnpm](https://pnpm.io/installation) for the frontend. Follow the suggested instructions or use your packaege manager like below

  ```shell
  # macOS
  brew install node
  npm install -g pnpm

  # Windows
  scoop install nodejs
  npm install -g pnpm
  ```

## Running

- Run Temporal on port 7233 (the port can be changed in [main.go](./backend/main.go)). The UI will be available at http://localhost:8233

  ```shell
  temporal server start-dev
  ```

- Run the Go backend on port 8080

  ```shell
  cd backend
  go run .
  ```

- Optionally, run the React frontend on port 5173 (default for Vite): https://localhost:5173
  ```shell
  cd frontend
  pnpm install
  pnpm run dev
  ```
