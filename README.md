# SwiftCSV

SwiftCSV is a high-throughput Go library for streaming CSV parsing and writing. It adheres to RFC 4180, keeps allocations low for large inputs, and exposes precise error information for malformed data.

## Features

- Streaming CSV reader with custom field and quote separators and minimal copying.
- Buffered CSV writer with configurable delimiters, newline policy, and forced quoting.
- Structured error reporting via `ParseError`, `ErrBareQuote`, `ErrUnterminatedQuote`, and `ErrorFieldCount`.
- Optional record reuse (`Reader.ReuseRecord`) and automatic width enforcement (`Reader.FieldsPerRecord`).
- Benchmarks, fuzz targets, and table-driven unit tests for regression protection.

## Getting Started

The module path is `swiftcsv`. Import it directly when working inside this repository or adjust the module path to match your fork or remote.

### Reader quick start

```go
package main

import (
    "fmt"
    "io"
    "strings"
    "swiftcsv"
)

func main() {
    reader := swiftcsv.NewReader(strings.NewReader("name,price\nWidget,12.50\n"))

    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            panic(err)
        }
        fmt.Println(record)
    }
}
```

Enable `reader.ReuseRecord = true` to reuse backing storage between calls; copy any fields you need to keep before the next `Read`. Set `reader.FieldsPerRecord` to a positive integer to enforce a uniform number of columns and receive `ErrorFieldCount` when records diverge.

### Writer quick start

```go
package main

import (
    "bytes"
    "log"
    "swiftcsv"
)

func main() {
    var buf bytes.Buffer
    writer := swiftcsv.NewWriter(&buf)
    defer func() {
        if err := writer.Flush(); err != nil {
            log.Fatal(err)
        }
    }()

    if err := writer.Write([]string{"name", "price"}); err != nil {
        log.Fatal(err)
    }
    if err := writer.Write([]string{"Widget", "12.50"}); err != nil {
        log.Fatal(err)
    }
}
```

Configure `writer.AlwaysQuote` to force quoting, `writer.UseCRLF` for Windows-style line endings, or swap `writer.Comma` / `writer.Quote` for TSV and semicolon-separated formats. Call `writer.WriteAll` for batch emission, `writer.Reset` to retarget an existing writer, and `writer.Error()` to inspect the first write failure.

## Error handling

`Reader.Read` and `Reader.ReadAll` return a `*swiftcsv.ParseError` when malformed input is detected. Use `errors.As` to obtain the line and column numbers, or `errors.Is` with `ErrBareQuote`, `ErrUnterminatedQuote`, and `ErrorFieldCount` to branch on error kinds.

The writer caches the first `Write`, `WriteAll`, or `Flush` failure. Subsequent operations return that error until `Reset` installs a fresh destination.

## Benchmarks and tests

Run the test suite and benchmarks with Go's standard tooling:

```sh
go test ./...
go test -run=^$ -bench . -benchmem
```

`reader_fuzz_test.go` contains a Go fuzz target that exercises edge cases in the parser. Fuzzing requires Go 1.20+:

```sh
go test -run=^$ -fuzz=FuzzReader -fuzztime=10s
```

## Examples

- `examples/main.go` demonstrates interactive parsing with custom delimiters.
- `example2/main.go` streams a large CSV file, showing how to discard BOMs and resume parsing.

## License

SwiftCSV is available under the MIT License. See `LICENSE.md` for the full text.
