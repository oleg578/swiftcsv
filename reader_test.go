package swiftcsv

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestReaderReadRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		comma byte
		quote byte
		reuse bool
		want  [][]string
	}{
		{
			name:  "basicRecords",
			input: "one,two\nthree,four\n",
			want: [][]string{
				{"one", "two"},
				{"three", "four"},
			},
		},
		{
			name:  "finalRecordWithoutTerminator",
			input: "alpha,beta,gamma",
			want: [][]string{
				{"alpha", "beta", "gamma"},
			},
		},
		{
			name:  "windowsLineEndings",
			input: "a,b\r\nc,d\r\n",
			want: [][]string{
				{"a", "b"},
				{"c", "d"},
			},
		},
		{
			name:  "quotedComma",
			input: "a,\"b,b\",c\n",
			want: [][]string{
				{"a", "b,b", "c"},
			},
		},
		{
			name:  "escapedQuote",
			input: "a,\"b\"\"c\",d\n",
			want: [][]string{
				{"a", "b\"c", "d"},
			},
		},
		{
			name:  "embeddedNewline",
			input: "a,\"b\nc\",d\n",
			want: [][]string{
				{"a", "b\nc", "d"},
			},
		},
		{
			name:  "emptyFields",
			input: ",,\n",
			want: [][]string{
				{"", "", ""},
			},
		},
		{
			name:  "customComma",
			input: "left;right\nup;down\n",
			comma: ';',
			want: [][]string{
				{"left", "right"},
				{"up", "down"},
			},
		},
		{
			name:  "customQuote",
			input: "alpha,'beta''gamma',delta\n",
			quote: '\'',
			want: [][]string{
				{"alpha", "beta'gamma", "delta"},
			},
		},
		{
			name:  "reuseRecord",
			input: "left,right\nup,down\n",
			reuse: true,
			want: [][]string{
				{"left", "right"},
				{"up", "down"},
			},
		},
		{
			name:  "quotedEOF",
			input: "\"quoted\"",
			want: [][]string{
				{"quoted"},
			},
		},
		{
			name:  "carriageReturnEOF",
			input: "one\rtwo",
			want: [][]string{
				{"one"},
				{"two"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := NewReader(strings.NewReader(tc.input))
			if tc.comma != 0 {
				r.Comma = tc.comma
			}
			if tc.quote != 0 {
				r.Quote = tc.quote
			}
			r.ReuseRecord = tc.reuse

			var records [][]string
			for {
				rec, err := r.Read()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatalf("Read() returned unexpected error: %v", err)
				}
				records = append(records, cloneStrings(rec))
			}

			if !reflect.DeepEqual(records, tc.want) {
				t.Fatalf("Read() records mismatch:\n got: %#v\nwant: %#v", records, tc.want)
			}
		})
	}
}

func TestReaderReuseRecord(t *testing.T) {
	t.Parallel()

	r := NewReader(strings.NewReader("alpha\nbeta\n"))
	r.ReuseRecord = true

	first, err := r.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	second, err := r.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("unexpected slice lengths: first=%d second=%d", len(first), len(second))
	}
	if &first[0] != &second[0] {
		t.Fatalf("expected backing slice to be reused")
	}
	if second[0] != "beta" || first[0] != "beta" {
		t.Fatalf("expected both slices to reflect latest record, got first=%q second=%q", first[0], second[0])
	}

	if _, err := r.Read(); !errors.Is(err, io.EOF) {
		t.Fatalf("Read() expected io.EOF, got %v", err)
	}
}

func TestReaderReuseRecordDisabled(t *testing.T) {
	t.Parallel()

	r := NewReader(strings.NewReader("alpha\nbeta\n"))

	first, err := r.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	second, err := r.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("unexpected slice lengths: first=%d second=%d", len(first), len(second))
	}
	if &first[0] == &second[0] {
		t.Fatalf("expected distinct backing slices when ReuseRecord is disabled")
	}
	if first[0] != "alpha" || second[0] != "beta" {
		t.Fatalf("unexpected record values: first=%q second=%q", first[0], second[0])
	}
}

func TestReaderErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		err    error
		line   int
		column int
	}{
		{
			name:   "bareQuote",
			input:  "a\"b,c\n",
			err:    ErrBareQuote,
			line:   1,
			column: 2,
		},
		{
			name:   "unterminatedQuoteSameLine",
			input:  "\"value",
			err:    ErrUnterminatedQuote,
			line:   1,
			column: 7,
		},
		{
			name:   "unterminatedQuoteMultiLine",
			input:  "\"alpha\nbeta",
			err:    ErrUnterminatedQuote,
			line:   2,
			column: 5,
		},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := NewReader(strings.NewReader(tc.input))
			_, err := r.Read()
			if err == nil {
				t.Fatalf("Read() expected error %v, got nil", tc.err)
			}

			var perr *ParseError
			if !errors.As(err, &perr) {
				t.Fatalf("Read() returned error %T, want *ParseError", err)
			}
			if !errors.Is(perr.Err, tc.err) {
				t.Fatalf("ParseError.Err = %v, want %v", perr.Err, tc.err)
			}
			if perr.Line != tc.line || perr.Column != tc.column {
				t.Fatalf("ParseError location = line %d column %d, want line %d column %d", perr.Line, perr.Column, tc.line, tc.column)
			}
		})
	}
}

func TestReaderReadAll(t *testing.T) {
	t.Parallel()

	const input = "a,b,c\n\"d\",\"e,f\",\"g\"\"h\"\nlast,row,\n"
	want := [][]string{
		{"a", "b", "c"},
		{"d", "e,f", "g\"h"},
		{"last", "row", ""},
	}

	r := NewReader(strings.NewReader(input))

	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("ReadAll() records mismatch:\n got: %#v\nwant: %#v", records, want)
	}
}

func TestReaderReadAllError(t *testing.T) {
	t.Parallel()

	r := NewReader(strings.NewReader("a,\"b\n"))

	records, err := r.ReadAll()
	if records != nil {
		t.Fatalf("ReadAll() returned records %+v, want nil on error", records)
	}
	if err == nil {
		t.Fatalf("ReadAll() expected error, got nil")
	}
	var perr *ParseError
	if !errors.As(err, &perr) {
		t.Fatalf("ReadAll() error type %T, want *ParseError", err)
	}
	if !errors.Is(perr.Err, ErrUnterminatedQuote) {
		t.Fatalf("ReadAll() error = %v, want ErrUnterminatedQuote", perr.Err)
	}
}

func TestParseErrorMethods(t *testing.T) {
	t.Parallel()

	err := &ParseError{Line: 3, Column: 7, Err: ErrBareQuote}
	if got := err.Error(); got == "" || !strings.Contains(got, "line 3") || !strings.Contains(got, "column 7") {
		t.Fatalf("Error() returned %q, want descriptive output", got)
	}
	if !errors.Is(err, ErrBareQuote) {
		t.Fatalf("ParseError should unwrap to ErrBareQuote")
	}
	if !errors.Is(err.Unwrap(), ErrBareQuote) {
		t.Fatalf("Unwrap() should return ErrBareQuote")
	}

	var nilErr *ParseError
	if nilErr.Error() != "" {
		t.Fatalf("nil ParseError should return empty string")
	}
	if nilErr.Unwrap() != nil {
		t.Fatalf("nil ParseError should return nil from Unwrap")
	}
}

func TestReaderFieldsPerRecord(t *testing.T) {
	t.Parallel()

	t.Run("autoDetectFirstRecord", func(t *testing.T) {
		t.Parallel()

		r := NewReader(strings.NewReader("a,b\nc,d\n"))

		record, err := r.Read()
		if err != nil {
			t.Fatalf("Read() error = %v, want nil", err)
		}
		if len(record) != 2 {
			t.Fatalf("Read() record length = %d, want 2", len(record))
		}
		if r.FieldsPerRecord != 2 {
			t.Fatalf("FieldsPerRecord = %d, want 2", r.FieldsPerRecord)
		}

		if _, err := r.Read(); err != nil {
			t.Fatalf("Read() second record error = %v, want nil", err)
		}
	})

	t.Run("mismatchReturnsError", func(t *testing.T) {
		t.Parallel()

		r := NewReader(strings.NewReader("x,y\n1,2,3\n"))
		r.FieldsPerRecord = 2

		if _, err := r.Read(); err != nil {
			t.Fatalf("Read() first record error = %v, want nil", err)
		}

		record, err := r.Read()
		if !errors.Is(err, ErrorFieldCount) {
			t.Fatalf("Read() error = %v, want ErrorFieldCount", err)
		}
		if len(record) != 3 {
			t.Fatalf("Read() record length = %d, want 3", len(record))
		}
	})
}

func TestNewReaderNilPanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("NewReader should panic on nil reader")
		}
	}()
	NewReader(nil)
}

func cloneStrings(rec []string) []string {
	out := make([]string, len(rec))
	for i, s := range rec {
		out[i] = string([]byte(s))
	}
	return out
}
