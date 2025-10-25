package swiftcsv

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestWriterWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		records [][]string
		config  func(*Writer)
		want    string
	}{
		{
			name:    "basic",
			records: [][]string{{"a", "b", "c"}},
			want:    "a,b,c\n",
		},
		{
			name: "multipleRecords",
			records: [][]string{
				{"alpha", "beta"},
				{"gamma", "delta"},
			},
			want: "alpha,beta\ngamma,delta\n",
		},
		{
			name:    "emptyField",
			records: [][]string{{"", "b"}},
			want:    ",b\n",
		},
		{
			name:    "commaForcesQuote",
			records: [][]string{{"alpha,beta"}},
			want:    "\"alpha,beta\"\n",
		},
		{
			name: "quoteEscaping",
			records: [][]string{
				{"he said \"hello\"", "plain"},
			},
			want: "\"he said \"\"hello\"\"\",plain\n",
		},
		{
			name: "newlineForcesQuote",
			records: [][]string{
				{"multi\nline", "z"},
			},
			want: "\"multi\nline\",z\n",
		},
		{
			name: "alwaysQuote",
			records: [][]string{
				{"alpha", "beta"},
			},
			config: func(w *Writer) {
				w.AlwaysQuote = true
			},
			want: "\"alpha\",\"beta\"\n",
		},
		{
			name: "customComma",
			records: [][]string{
				{"a;b", "c"},
			},
			config: func(w *Writer) {
				w.Comma = ';'
			},
			want: "\"a;b\";c\n",
		},
		{
			name: "customQuote",
			records: [][]string{
				{"alpha'beta", "plain"},
			},
			config: func(w *Writer) {
				w.Quote = '\''
			},
			want: "'alpha''beta',plain\n",
		},
		{
			name: "useCRLF",
			records: [][]string{
				{"a"},
				{"b"},
			},
			config: func(w *Writer) {
				w.UseCRLF = true
			},
			want: "a\r\nb\r\n",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			w := NewWriter(&buf)
			if tc.config != nil {
				tc.config(w)
			}
			for _, rec := range tc.records {
				if err := w.Write(rec); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
			}
			if err := w.Flush(); err != nil {
				t.Fatalf("Flush() error = %v", err)
			}
			if got := buf.String(); got != tc.want {
				t.Fatalf("unexpected output:\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestWriterWriteAll(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := NewWriter(&buf)

	records := [][]string{
		{"alpha", "beta"},
		{"gamma", "delta"},
	}

	if err := w.WriteAll(records); err != nil {
		t.Fatalf("WriteAll() error = %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	want := "alpha,beta\ngamma,delta\n"
	if got := buf.String(); got != want {
		t.Fatalf("unexpected output got %q want %q", got, want)
	}
}

func TestWriterReset(t *testing.T) {
	t.Parallel()

	var buf1 bytes.Buffer
	var buf2 bytes.Buffer

	var w Writer
	w.Reset(&buf1)

	if err := w.Write([]string{"a"}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if got := buf1.String(); got != "a\n" {
		t.Fatalf("unexpected buf1 contents %q", got)
	}

	w.Comma = ';'
	w.UseCRLF = true
	w.Reset(&buf2)
	if err := w.Write([]string{"x", "y"}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if got := buf2.String(); got != "x;y\r\n" {
		t.Fatalf("unexpected buf2 contents %q", got)
	}
}

type flushFailWriter struct {
	fail error
}

func (f *flushFailWriter) Write([]byte) (int, error) {
	return 0, f.fail
}

func TestWriterFlushError(t *testing.T) {
	t.Parallel()

	exp := errors.New("flush failed")
	w := NewWriter(&flushFailWriter{fail: exp})

	if err := w.Write([]string{"a"}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := w.Flush(); !errors.Is(err, exp) {
		t.Fatalf("expected flush error %v, got %v", exp, err)
	}
	if err := w.Write([]string{"b"}); !errors.Is(err, exp) {
		t.Fatalf("Write() should return stored error %v, got %v", exp, err)
	}
}

func TestWriterErrorMethod(t *testing.T) {
	t.Parallel()

	w := NewWriter(&strings.Builder{})
	if err := w.Error(); err != nil {
		t.Fatalf("expected nil error from fresh writer, got %v", err)
	}

	exp := errors.New("flush failed")
	w.Reset(&flushFailWriter{fail: exp})
	if err := w.Write([]string{"a"}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := w.Flush(); !errors.Is(err, exp) {
		t.Fatalf("expected flush error %v, got %v", exp, err)
	}
	if err := w.Error(); !errors.Is(err, exp) {
		t.Fatalf("Error() should return %v, got %v", exp, err)
	}
}
