# 📦 logger-server

> A lightweight, fast, and extensible logging server built in Go, designed to collect, store, and query your logs efficiently. Works seamlessly with Fluent Bit to forward logs via HTTP.

---

## 🚀 Features

- 🔥 High-performance logging backend in Go  
- 🌐 HTTP API for log ingestion and querying  
- 🧩 Context-aware logs with JSON support  
- 🛡️ API key authentication support (planned)  
- 📦 Fluent Bit integration out of the box  
- ⚙️ Pagination, filtering by log level, and timestamp support  
- 🧪 Fully tested with coverage reports  
- 🐳 Ready for Docker deployment  

---

## 📚 Table of Contents

- [Getting Started](#getting-started)  
- [Installation](#installation)  
- [Configuration](#⚙️-configuration)  
- [Usage](#🏃-usage)  
- [API Reference](#📖-api-reference)  
- [Testing](#🧪-testing)  
- [License](#📄-license)  

---

## Getting Started

### Prerequisites

Make sure you have:

- Go 1.20+ installed 🐹  
- Fluent Bit installed and configured 🐦  
- (Optional) Docker installed if you want to run with containers 🐳  

### Installation

Clone the repo:

```bash
git clone https://github.com/rypi-dev/logger-server.git
cd logger-server
```

Install dependencies and build:
```bash
go mod download
go build -o logger-server cmd/main.go
```

Run the server:
```bash
./logger-server
```

By default, it listens on port 8080 for incoming logs.

### ⚙️ Configuration

Fluent Bit Integration
Fluent Bit is configured to forward logs as JSON via HTTP to the logger-server.

Sample Fluent Bit config snippet:

```ini
[INPUT]
    Name   http
    Listen 0.0.0.0
    Port   8888
    Format json
    Tag    incoming.log

[FILTER]
    Name   lua
    Match  incoming.*
    script lua/inject_header.lua
    call   add_log_level_header

[FILTER]
    Name   modify
    Match  incoming.*
    Add    stack multi-lang

[OUTPUT]
    Name   http
    Match  incoming.*
    Host   logger-server
    Port   8080
    URI    /log
    Format json
    Header Content-Type application/json
    Header X-Log-Level ${X-Log-Level}
```

## 🏃 Usage

### Sending logs
Send JSON logs to http://localhost:8080/log with the following payload:

```json
{
  "level": "INFO",
  "message": "User logged in",
  "timestamp": "2025-08-06T14:12:00Z",
  "context": {
    "user_id": 42
  }
}
```

### Querying logs
You can query logs with pagination and filtering by log level:

```pgsql
GET /logs?page=1&limit=50&level=ERROR
```

## 📖 API Reference

-   POST /log — Ingest a new log entry

-   GET /logs — Query logs with filters (page, limit, level)

Request and response formats follow JSON standards.


## 🧪 Testing

Run all tests and check coverage:

```bash
make ci
```

Generate HTML coverage report:

```bash
make report
```

Open `coverage.html` in your browser to visualize test coverage.

## 📄 License

This project is licensed under the MIT License — see the LICENSE file for details.

## 💬 Contact

For any questions or suggestions, open an issue or reach out directly on GitHub!