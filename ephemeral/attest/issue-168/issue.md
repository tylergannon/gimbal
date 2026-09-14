# Add a hello command

Add `cmd/hello/main.go`, a program that prints `hello` and a newline. Put the text in a function `greeting() string` and add a test in the same package that checks it returns `hello\n`. `go vet ./...` and `go test ./...` pass.
