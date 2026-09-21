# Contributing to Aethium

## Reporting bugs
Open an issue on GitHub with:
- Aethium version (aethium version)
- Your OS
- The .aeth code that caused the issue
- The error message

## Submitting changes
1. Fork the repo
2. Create a branch: git checkout -b my-feature
3. Make your changes
4. Run tests: go test ./...
5. Open a pull request

## Code style
- Follow standard Go formatting (gofmt)
- Add tests for new language features in pkg/engine/engine_test.go
- Keep compiler and VM changes in separate commits

## License
By contributing, you agree your contributions are licensed under AGPLv3.