# SteadyQ 🚀

SteadyQ is a modern, TUI-based load testing tool designed for developers who need a quick, visual, and interactive way to stress test HTTP endpoints.

## 🌟 Features

- **Interactive TUI**: Built with Bubble Tea for a terminal-based UI experience.
- **Dynamic Configuration**: Easily switch between `HTTP` requests and custom `Shell Scripts`.
- **Flexible Load Modes**:
  - **RPS (Open Loop)**: Target a specific Requests Per Second.
  - **Users (Closed Loop)**: Simulate fixed concurrent users with think time.
- **Real-time Dashboard**: Visualize latency, throughput, and errors live.

## 📦 Installation

Prerequisites: `Go 1.21+`

```bash
# Clone the repository
git clone https://github.com/yourusername/steadyq.git
cd steadyq

# Build the binary
go build -o steadyq .

# Move to path (Optional)
sudo mv steadyq /usr/local/bin/
```

## 🎮 Usage

Run the tool simply by executing the binary:

```bash
steadyq
```

### Key Bindings

| Key                 | Action                                |
| :------------------ | :------------------------------------ |
| `Ctrl+Left/Right`   | Switch Views (Runner, Dashboard)      |
| `Tab` / `Shift+Tab` | Navigate Fields                       |
| `Enter`             | Edit Field                            |
| `Space`             | Toggle Modes (RPS/Users, HTTP/Script) |
| `Ctrl+R`            | **Run** Test                          |
| `Ctrl+S`            | **Stop** Test                         |
| `Ctrl+D`            | Go to Dashboard                       |
| `Ctrl+Q`            | Quit                                  |

## 🛠 Configuration

### Request Types

- **HTTP**: Standard GET/POST requests.
- **Script**: Execute any shell command. Use `{{userID}}` and `{{chatID}}` as placeholders.

### Load Modes

- **RPS**: "Open Loop" testing. Tries to maintain target throughput.
- **Users**: "Closed Loop" testing. Waits for response + think time before next request.

## 📝 License

MIT
