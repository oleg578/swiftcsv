package swiftcsv

import (
	"bufio"
	"errors"
	"io"
)

var (
	errNilWriter      = errors.New("swiftcsv: writer is nil")
	errWriterNoTarget = errors.New("swiftcsv: writer destination cannot be nil")
)

// Writer provides high-throughput CSV emission with configurable delimiters and quoting rules.
type Writer struct {
	dst *bufio.Writer

	// Comma is the field delimiter. Default is ','.
	Comma byte
	// Quote is the quote character. Default is '"'.
	Quote byte
	// UseCRLF writes records terminated with \r\n when set.
	UseCRLF bool
	// AlwaysQuote forces quoting for all fields when enabled.
	AlwaysQuote bool

	err error
}

// NewWriter creates a new Writer with internal buffering tuned for bulk writes.
func NewWriter(w io.Writer) *Writer {
	if w == nil {
		panic(errWriterNoTarget.Error())
	}
	return &Writer{
		dst:   bufio.NewWriterSize(w, defaultBufferSize),
		Comma: ',',
		Quote: '"',
	}
}

// Reset updates the underlying writer while preserving the configuration flags.
func (w *Writer) Reset(dst io.Writer) {
	if w == nil {
		panic(errNilWriter.Error())
	}
	if dst == nil {
		panic(errWriterNoTarget.Error())
	}
	if w.dst == nil {
		w.dst = bufio.NewWriterSize(dst, defaultBufferSize)
	} else {
		w.dst.Reset(dst)
	}
	w.err = nil
}

// Write emits a single CSV record. The record is terminated with the configured newline sequence.
func (w *Writer) Write(record []string) error {
	if w == nil {
		return errNilWriter
	}
	if w.dst == nil {
		return errWriterNoTarget
	}
	if w.err != nil {
		return w.err
	}

	comma := w.Comma
	if comma == 0 {
		comma = ','
	}
	quote := w.Quote
	if quote == 0 {
		quote = '"'
	}

	for i := range record {
		if i > 0 {
			if err := w.dst.WriteByte(comma); err != nil {
				w.err = err
				return err
			}
		}
		if err := w.writeField(record[i], comma, quote); err != nil {
			w.err = err
			return err
		}
	}

	if w.UseCRLF {
		if _, err := w.dst.Write([]byte{'\r', '\n'}); err != nil {
			w.err = err
			return err
		}
	} else {
		if err := w.dst.WriteByte('\n'); err != nil {
			w.err = err
			return err
		}
	}
	return nil
}

// WriteAll writes multiple records, stopping at the first error.
func (w *Writer) WriteAll(records [][]string) error {
	if w == nil {
		return errNilWriter
	}
	for _, record := range records {
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return nil
}

// Flush flushes pending buffered data to the underlying writer.
func (w *Writer) Flush() error {
	if w == nil {
		return errNilWriter
	}
	if w.dst == nil {
		return errWriterNoTarget
	}
	if w.err != nil {
		return w.err
	}
	if err := w.dst.Flush(); err != nil {
		w.err = err
		return err
	}
	return nil
}

// Error reports the first error encountered by the writer.
func (w *Writer) Error() error {
	if w == nil {
		return errNilWriter
	}
	return w.err
}

func (w *Writer) writeField(field string, comma, quote byte) error {
	needsQuote := w.AlwaysQuote
	if !needsQuote {
		needsQuote = fieldNeedsQuote(field, comma, quote)
	}
	if !needsQuote {
		_, err := w.dst.WriteString(field)
		return err
	}
	if err := w.dst.WriteByte(quote); err != nil {
		return err
	}

	start := 0
	for i := 0; i < len(field); i++ {
		if field[i] == quote {
			if start < i {
				if _, err := w.dst.WriteString(field[start:i]); err != nil {
					return err
				}
			}
			if _, err := w.dst.Write([]byte{quote, quote}); err != nil {
				return err
			}
			start = i + 1
		}
	}
	if start < len(field) {
		if _, err := w.dst.WriteString(field[start:]); err != nil {
			return err
		}
	}
	if err := w.dst.WriteByte(quote); err != nil {
		return err
	}
	return nil
}

func fieldNeedsQuote(field string, comma, quote byte) bool {
	for i := 0; i < len(field); i++ {
		switch field[i] {
		case quote, comma, '\n', '\r':
			return true
		}
	}
	return false
}
