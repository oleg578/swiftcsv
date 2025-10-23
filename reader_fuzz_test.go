package swiftcsv

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func FuzzReaderConsistency(f *testing.F) {
	seeds := []string{
		"",
		"a,b,c\n",
		"a,\"b,b\",c\n",
		"a,\"b\nc\",d\n",
		"\"unterminated\n",
		"a\"b,c\n",
		"one\r\ntwo\r\n",
		"trailing,newline\n",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 1<<12 {
			t.Skip()
		}

		recordsManual, errManual := readRecordsSequential(input, false)
		recordsReuse, errReuse := readRecordsSequential(input, true)
		recordsAll, errAll := readRecordsAll(input)

		if !sameReaderError(errManual, errReuse) {
			t.Fatalf("reuse mismatch: errManual=%v errReuse=%v input=%q", errManual, errReuse, truncateForMessage(input))
		}
		if !sameReaderError(errManual, errAll) {
			t.Fatalf("ReadAll mismatch: errManual=%v errAll=%v input=%q", errManual, errAll, truncateForMessage(input))
		}

		if errManual == nil {
			if !recordsEqual(recordsManual, recordsReuse) {
				t.Fatalf("records mismatch with reuse:\nmanual=%v\nreuse=%v\ninput=%q", recordsManual, recordsReuse, truncateForMessage(input))
			}
			if !recordsEqual(recordsManual, recordsAll) {
				t.Fatalf("records mismatch with ReadAll:\nmanual=%v\nreadAll=%v\ninput=%q", recordsManual, recordsAll, truncateForMessage(input))
			}
		}
	})
}

func readRecordsSequential(input string, reuse bool) ([][]string, error) {
	r := NewReader(strings.NewReader(input))
	r.ReuseRecord = reuse

	var out [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, cloneStrings(rec))
	}
}

func readRecordsAll(input string) ([][]string, error) {
	r := NewReader(strings.NewReader(input))
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	copied := make([][]string, len(records))
	for i, rec := range records {
		copied[i] = cloneStrings(rec)
	}
	return copied, nil
}

func sameReaderError(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	sigA, lineA, colA := readerErrorSignature(a)
	sigB, lineB, colB := readerErrorSignature(b)
	return sigA == sigB && lineA == lineB && colA == colB
}

func readerErrorSignature(err error) (sig string, line int, column int) {
	var perr *ParseError
	if errors.As(err, &perr) {
		switch {
		case errors.Is(perr.Err, ErrBareQuote):
			return "bare_quote", perr.Line, perr.Column
		case errors.Is(perr.Err, ErrUnterminatedQuote):
			return "unterminated_quote", perr.Line, perr.Column
		default:
			return perr.Err.Error(), perr.Line, perr.Column
		}
	}
	return err.Error(), 0, 0
}

func recordsEqual(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

func truncateForMessage(s string) string {
	const max = 256
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
