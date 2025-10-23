// # SwiftCSV: A High-Throughput Streaming CSV Library for Go
//
// SwiftCSV is a high-throughput Go library for streaming CSV parsing and writing. It adheres to RFC 4180, keeps allocations low for large inputs, and exposes precise error information for malformed data.
//
// # Features
//
// - Streaming CSV reader with custom field and quote separators and minimal copying.
// - Buffered CSV writer with configurable delimiters, newline policy, and forced quoting.
// - Structured error reporting via `ParseError`, `ErrBareQuote`, `ErrUnterminatedQuote`, and `ErrorFieldCount`.
// - Optional record reuse (`Reader.ReuseRecord`) and automatic width enforcement (`Reader.FieldsPerRecord`).
// - Benchmarks, fuzz targets, and table-driven unit tests for regression protection.
//
// # Getting Started
//
// The module path is `swiftcsv`. Import it directly when working inside this repository or adjust the module path to match your fork or remote.
package swiftcsv