package swiftcsv

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"unsafe"
)

const defaultBufferSize = 1 << 10 // 1024 bytes

var (
	// ErrBareQuote is returned when an unexpected quote is found in an unquoted field.
	ErrBareQuote = errors.New("swiftcsv: bare quote in non-quoted field")
	// ErrUnterminatedQuote is returned when a quoted field is not closed before EOF or record end.
	ErrUnterminatedQuote = errors.New("swiftcsv: unterminated quoted field")
	// ErrorFieldCount is returned when a record contains an unexpected number of fields.
	ErrorFieldCount = errors.New("swiftcsv: wrong number of fields")
)

// ParseError contains location information for CSV parsing errors.
type ParseError struct {
	Line   int
	Column int
	Err    error
}

// Error formats the parse error message with the stored line, column, and Err values.
func (e *ParseError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("swiftcsv: parse error on line %d, column %d: %v", e.Line, e.Column, e.Err)
}

// Unwrap returns the underlying Err so ParseError participates in errors.Unwrap.
func (e *ParseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Reader provides high-performance CSV parsing with support for customizable delimiters.
type Reader struct {
	src io.Reader

	// Comma is the field delimiter. Default is ','.
	Comma byte
	// Quote is the quote character. Default is '"'.
	Quote byte
	// ReuseRecord indicates whether Read should reuse the backing array of the returned slice.
	ReuseRecord bool
	// FieldsPerRecord expects each record to contain this many fields. Zero captures the width of the first record.
	FieldsPerRecord int

	buf    []byte
	bufPos int
	bufLen int
	bufErr error

	record      []string
	dataBuf     []byte
	fieldBounds []int
	finished    bool
	line        int
}

// NewReader creates a Reader that consumes CSV data from r, panicking if r is nil,
// and initialises internal buffers sized for high-throughput parsing. It returns
// a pointer to the configured Reader.
func NewReader(r io.Reader) *Reader {
	if r == nil {
		panic("swiftcsv: reader source cannot be nil")
	}

	return &Reader{
		src:         r,
		Comma:       ',',
		Quote:       '"',
		buf:         make([]byte, defaultBufferSize),
		record:      make([]string, 0, 16),
		dataBuf:     make([]byte, 0, 512),
		fieldBounds: make([]int, 0, 32),
		line:        1,
	}
}

// Read parses the next CSV record from the underlying stream. It returns dst containing
// the field values (which may reuse internal storage when ReuseRecord is true) and an err
// indicating success or failure; io.EOF signals that no more records remain.
func (r *Reader) Read() (dst []string, err error) {
	if r == nil || r.src == nil {
		return nil, io.EOF
	}
	if r.finished {
		return nil, io.EOF
	}

	comma := r.Comma
	if comma == 0 {
		comma = ','
	}
	quote := r.Quote
	if quote == 0 {
		quote = '"'
	}

	// Reset state for assembling the next record, reusing slices when allowed.
	if r.ReuseRecord {
		r.record = r.record[:0]
	} else {
		r.record = nil
	}
	r.dataBuf = r.dataBuf[:0]
	r.fieldBounds = r.fieldBounds[:0]

	inQuotes := false
	sawQuotedField := false
	column := 1
	fieldStart := 0

	for {
		// Ensure the working buffer has data before parsing the next byte.
		if r.bufPos >= r.bufLen {
			if r.bufErr != nil {
				curColumn := column
				err := r.bufErr
				r.bufErr = nil
				if err == io.EOF {
					// Unterminated quotes at EOF are invalid.
					if inQuotes {
						r.finished = true
						return nil, r.wrapError(curColumn, ErrUnterminatedQuote)
					}
					// Flush a trailing field if data ended without a newline.
					if len(r.fieldBounds) > 0 || len(r.dataBuf) > 0 || sawQuotedField {
						r.fieldBounds = append(r.fieldBounds, fieldStart, len(r.dataBuf))
						r.finished = true
						return r.buildRecord()
					}
					r.finished = true
					return nil, io.EOF
				}
				return nil, err
			}

			// Pull the next chunk from the source.
			n, err := r.src.Read(r.buf)
			if n == 0 {
				if err != nil {
					r.bufErr = err
				}
				continue
			}
			r.bufPos = 0
			r.bufLen = n
			r.bufErr = err
		}

		if !inQuotes {
			// Fast-path plain bytes until a quote or delimiter is encountered.
			data := r.buf[r.bufPos:r.bufLen]
			if len(data) == 0 {
				continue
			}

			quoteIdx := bytes.IndexByte(data, quote)
			switch {
			case quoteIdx == -1:
				// Consume plain bytes, returning early if we closed a record.
				recordDone, err := r.consumePlain(&column, &fieldStart, &sawQuotedField)
				if err != nil {
					return nil, err
				}
				if recordDone {
					return r.buildRecord()
				}
				if r.bufPos >= r.bufLen {
					continue
				}
			case quoteIdx > 0:
				// Temporarily limit the buffer to process plain bytes up to the quote.
				originalLen := r.bufLen
				r.bufLen = r.bufPos + quoteIdx
				recordDone, err := r.consumePlain(&column, &fieldStart, &sawQuotedField)
				r.bufLen = originalLen
				if err != nil {
					return nil, err
				}
				if recordDone {
					return r.buildRecord()
				}
				if r.bufPos >= r.bufLen {
					continue
				}
			}
		}

		curColumn := column
		b := r.buf[r.bufPos]
		r.bufPos++

		if inQuotes {
			if b == quote {
				// Double quote inside quotes represents an escaped quote.
				next, err := r.peekByte()
				if err == nil && next == quote {
					r.bufPos++
					r.dataBuf = append(r.dataBuf, quote)
					column = curColumn + 2
					continue
				}
				if err != nil && err != io.EOF {
					return nil, err
				}
				inQuotes = false
				column = curColumn + 1
				continue
			}
			if b == '\n' {
				// Track logical line numbers for embedded newlines.
				r.dataBuf = append(r.dataBuf, b)
				r.line++
				column = 1
				continue
			}

			start := r.bufPos - 1
			run := 1
			if r.bufPos < r.bufLen {
				data := r.buf[r.bufPos:r.bufLen]
				for i := 0; i < len(data); i++ {
					c := data[i]
					if c == quote || c == '\n' {
						break
					}
					run++
				}
				r.bufPos += run - 1
			}
			column = curColumn + run
			// Append contiguous plain bytes within the quoted field.
			r.dataBuf = append(r.dataBuf, r.buf[start:start+run]...)
			continue
		}

		switch b {
		case comma:
			r.fieldBounds = append(r.fieldBounds, fieldStart, len(r.dataBuf))
			fieldStart = len(r.dataBuf)
			sawQuotedField = false
			column = curColumn + 1
		case '\n':
			r.fieldBounds = append(r.fieldBounds, fieldStart, len(r.dataBuf))
			sawQuotedField = false
			r.line++
			column = 1
			return r.buildRecord()
		case '\r':
			next, err := r.peekByte()
			if err == nil && next == '\n' {
				r.bufPos++
			}
			if err != nil && err != io.EOF {
				return nil, err
			}
			r.fieldBounds = append(r.fieldBounds, fieldStart, len(r.dataBuf))
			sawQuotedField = false
			r.line++
			column = 1
			return r.buildRecord()
		case quote:
			// A quote starts a quoted field only if we have not buffered any characters yet.
			if len(r.dataBuf) == fieldStart && !sawQuotedField {
				inQuotes = true
				sawQuotedField = true
				column = curColumn + 1
				continue
			}
			return nil, r.wrapError(curColumn, ErrBareQuote)
		default:
			start := r.bufPos - 1
			run := 1
			if r.bufPos < r.bufLen {
				data := r.buf[r.bufPos:r.bufLen]
				for i := 0; i < len(data); i++ {
					c := data[i]
					if c == comma || c == '\n' || c == '\r' || c == quote {
						break
					}
					run++
				}
				r.bufPos += run - 1
			}
			column = curColumn + run
			// Copy consecutive plain bytes before the next delimiter.
			r.dataBuf = append(r.dataBuf, r.buf[start:start+run]...)
		}
	}
}

// ReadAll exhausts the reader, repeatedly calling Read to collect records until io.EOF
// and returning the accumulated records slice plus the first non-EOF error encountered.
func (r *Reader) ReadAll() (records [][]string, err error) {
	for {
		record, err := r.Read()
		if err == io.EOF {
			return records, nil
		}
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
}

// buildRecord maps the accumulated fieldBounds onto the data buffer, respecting ReuseRecord,
// and returns the materialised []string representing the current record.
func (r *Reader) buildRecord() ([]string, error) {
	fieldCount := len(r.fieldBounds) / 2

	var recordStr string
	if r.ReuseRecord {
		if len(r.dataBuf) == 0 {
			recordStr = ""
		} else {
			// Zero-copy string construction so fields can share a single backing buffer.
			recordStr = unsafe.String(unsafe.SliceData(r.dataBuf), len(r.dataBuf))
		}
		if cap(r.record) < fieldCount {
			r.record = make([]string, fieldCount)
		}
		r.record = r.record[:fieldCount]
	} else {
		recordStr = string(r.dataBuf)
		r.record = make([]string, fieldCount)
	}

	for i := 0; i < fieldCount; i++ {
		start := r.fieldBounds[2*i]
		end := r.fieldBounds[2*i+1]
		r.record[i] = recordStr[start:end]
	}

	if r.FieldsPerRecord <= 0 {
		r.FieldsPerRecord = len(r.record)
		return r.record, nil
	}
	if len(r.record) != r.FieldsPerRecord {
		return r.record, ErrorFieldCount
	}
	return r.record, nil
}

// wrapError attaches the current line and supplied column to err, producing a *ParseError.
func (r *Reader) wrapError(column int, err error) error {
	return &ParseError{Line: r.line, Column: column, Err: err}
}

// consumePlain consumes unquoted field data, updating *column, *fieldStart, and *sawQuotedField.
// It reports whether a record terminator was seen and returns any read error encountered.
func (r *Reader) consumePlain(column *int, fieldStart *int, sawQuotedField *bool) (bool, error) {
	comma := r.Comma
	if comma == 0 {
		comma = ','
	}

	for {
		if r.bufPos >= r.bufLen {
			return false, nil
		}

		// Locate the closest delimiter or record terminator within the buffered bytes.
		data := r.buf[r.bufPos:r.bufLen]
		idxComma := bytes.IndexByte(data, comma)
		idxNewline := bytes.IndexByte(data, '\n')
		idxCR := bytes.IndexByte(data, '\r')

		next := len(data)
		delim := byte(0)

		if idxComma >= 0 && idxComma < next {
			next = idxComma
			delim = comma
		}
		if idxNewline >= 0 && idxNewline < next {
			next = idxNewline
			delim = '\n'
		}
		if idxCR >= 0 && idxCR < next {
			next = idxCR
			delim = '\r'
		}

		// Append the plain run preceding the delimiter and advance position counters.
		if next > 0 {
			r.dataBuf = append(r.dataBuf, data[:next]...)
			r.bufPos += next
			*column += next
		}

		if delim == 0 {
			return false, nil
		}

		r.bufPos++
		switch delim {
		case comma:
			r.fieldBounds = append(r.fieldBounds, *fieldStart, len(r.dataBuf))
			*fieldStart = len(r.dataBuf)
			*sawQuotedField = false
			*column = *column + 1
		case '\n':
			r.fieldBounds = append(r.fieldBounds, *fieldStart, len(r.dataBuf))
			*sawQuotedField = false
			r.line++
			*column = 1
			return true, nil
		case '\r':
			// Support CRLF by peeking ahead for '\n' and consuming it together.
			nextByte, err := r.peekByte()
			if err == nil && nextByte == '\n' {
				r.bufPos++
			} else if err != nil && err != io.EOF {
				return false, err
			}
			r.fieldBounds = append(r.fieldBounds, *fieldStart, len(r.dataBuf))
			*sawQuotedField = false
			r.line++
			*column = 1
			return true, nil
		}
	}
}

// peekByte returns the next buffered byte (refilling from src as needed) and propagates any read error.
func (r *Reader) peekByte() (byte, error) {
	for {
		if r.bufPos < r.bufLen {
			return r.buf[r.bufPos], nil
		}
		if r.bufErr != nil {
			return 0, r.bufErr
		}

		n, err := r.src.Read(r.buf)
		if n == 0 && err != nil {
			return 0, err
		}
		if n == 0 {
			continue
		}
		r.bufPos = 0
		r.bufLen = n
		r.bufErr = err
	}
}
