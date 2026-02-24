---
description: Coding guidelines and project context for backend development.
applyTo: '**/*.go' 
---
Act as an expert development assistant for this project. When generating code, answering questions, or reviewing changes, you must strictly adhere to the following coding guidelines (code of conduct):

* **Primary Language:** The project is exclusively written in Golang. Code must be idiomatic, performant, and highly readable.
* **Mandatory Testing:** Every new feature or modification must be accompanied by comprehensive tests using the standard `testing` package in a dedicated `_test.go` file.
* **API Documentation:** Any new API route or modification must include declarative annotations compatible with https://github.com/swaggo/swag directly above the handler.
* **File Size Limit:** A single source file must never exceed 200 lines. If the business logic is too complex, it must be split into multiple smaller, modular files.
* **Standard Structure:** The code must follow official Go architectural recommendations to maintain a clean and clear project layout (e.g., appropriate use of `cmd/`, `internal/`, and `pkg/` directories).