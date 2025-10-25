![Screenshot](./thumbnail.png)

# Conversion CalHacks Demo - EternalGameOfLife

A tutorial for using signals in [Temporal](https://docs.temporal.io/evaluate/why-temporal) to coordinate an event-driven UI for a durable, long-lived [game of life](https://en.wikipedia.org/wiki/Conway%27s_Game_of_Life).

The game of life is composed of some very simple, deterministic rules:

1. Any live cell with fewer than two live neighbours dies, as if by underpopulation.
2. Any live cell with two or three live neighbours lives on to the next generation.
3. Any live cell with more than three live neighbours dies, as if by overpopulation.
4. Any dead cell with exactly three live neighbours becomes a live cell, as if by reproduction.

> [!WARNING]
> This is NOT a good production use case for Temporal. We are simply exploring how Temporal can be used for durable, long-running jobs!

## Goals
Implement the `splatter` method, which allows a visitor to click and make a random amount of cells appear.
Implement the `toggle` signal, allowing the user to pause and unpase.

1. Create new selector callbacks on the main selector for each signal (splatter and toggle)
2. Implement the splatter activity for the splatter signal to your liking
3. Test durability (nuke the backend + refresh the frontend)

## Requirements

- Download [go](https://go.dev/dl/). Follow the suggested instructions or use your packaege manager like below

  ```shell
  # macOS
  brew install go

  # Windows
  scoop install go
  ```

- Download [Temporal](https://learn.temporal.io/getting_started/go/dev_environment). Follow the suggested instructions or use your package manager like below

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
- If you opt to not use the frontend, Decrease the board size to 40 X 40 and print to the terminal
