# NeoBase Desktop

NeoBase Desktop is a cross-platform desktop application built with [Wails](https://wails.io/) and Go, providing a native experience for the NeoBase AI Database Assistant.

## Features

- All the features of the web client, but in a native desktop application
- Offline authentication
- Native performance
- Cross-platform (Windows, macOS, Linux)

## Development

### Prerequisites

- Go 1.18+
- Node.js 16+
- npm or yarn
- Wails CLI

### Installation

1. Install Wails CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

2. Clone the repository:

```bash
git clone https://github.com/yourusername/neobase-desktop.git
cd neobase-desktop
```

3. Install dependencies:

```bash
cd frontend
npm install
cd ..
```

4. Run the application in development mode:

```bash
wails dev
```

### Building

To build the application for your current platform:

```bash
wails build
```

To build for a specific platform:

```bash
wails build -platform=windows/amd64
wails build -platform=darwin/universal
wails build -platform=linux/amd64
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
