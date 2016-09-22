package util

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CSVWriter reimplements the standard library csv.Writer adding AlwaysEncapsulate and QuoteEscape
type CSVWriter struct {
	Comma             rune
	UseCRLF           bool
	w                 *bufio.Writer
	AlwaysEncapsulate bool   // If the content should be encapsulated independent of its type
	QuoteEscape       string // String to use to escape a quote character
}

// NewCSVWriter instantiates a new instance of CSVWriter
func NewCSVWriter() *CSVWriter {
	return &CSVWriter{
		Comma:             ',',
		UseCRLF:           false,
		AlwaysEncapsulate: true,
		QuoteEscape:       `\`,
	}
}

// SetWriter allows you to change the writer (which is not directly exposed)
func (w *CSVWriter) SetWriter(writer io.Writer) {
	w.w = bufio.NewWriter(writer)
}

// Write writes a single CSV record to w along with any necessary quoting.
// A record is a slice of strings with each string being one field.
func (w *CSVWriter) Write(record []string) (err error) {
	for n, field := range record {
		if n > 0 {
			if _, err = w.w.WriteRune(w.Comma); err != nil {
				return
			}
		}

		if !w.fieldNeedsQuotes(field) {
			if _, err = w.w.WriteString(field); err != nil {
				return
			}
			continue
		}
		if err = w.w.WriteByte('"'); err != nil {
			return
		}

		for _, r1 := range field {
			switch r1 {
			case '"':
				_, err = w.w.WriteString(fmt.Sprintf(`%v"`, w.QuoteEscape))
			case '\r':
				if !w.UseCRLF {
					err = w.w.WriteByte('\r')
				}
			case '\n':
				if w.UseCRLF {
					_, err = w.w.WriteString("\r\n")
				} else {
					err = w.w.WriteByte('\n')
				}
			default:
				_, err = w.w.WriteRune(r1)
			}
			if err != nil {
				return
			}
		}

		if err = w.w.WriteByte('"'); err != nil {
			return
		}
	}

	if w.UseCRLF {
		_, err = w.w.WriteString("\r\n")
	} else {
		err = w.w.WriteByte('\n')
	}

	return
}

// Flush writes any buffered data to the underlying io.Writer.
// To check if an error occurred during the Flush, call Error.
func (w *CSVWriter) Flush() {
	w.w.Flush()
}

// Error reports any error that has occurred during a previous Write or Flush.
func (w *CSVWriter) Error() error {
	_, err := w.w.Write(nil)
	return err
}

// WriteAll writes multiple CSV records to w using Write and then calls Flush.
func (w *CSVWriter) WriteAll(records [][]string) (err error) {
	for _, record := range records {
		err = w.Write(record)
		if err != nil {
			return err
		}
	}
	return w.w.Flush()
}

// fieldNeedsQuotes reports whether our field must be enclosed in quotes.
// Fields with a Comma, fields with a quote or newline, and
// fields which start with a space must be enclosed in quotes.
// We used to quote empty strings, but we do not anymore (as of Go 1.4).
// The two representations should be equivalent, but Postgres distinguishes
// quoted vs non-quoted empty string during database imports, and it has
// an option to force the quoted behavior for non-quoted CSV but it has
// no option to force the non-quoted behavior for quoted CSV, making
// CSV with quoted empty strings strictly less useful.
// Not quoting the empty string also makes this package match the behavior
// of Microsoft Excel and Google Drive.
// For Postgres, quote the data terminating string `\.`.
func (w *CSVWriter) fieldNeedsQuotes(field string) bool {
	if w.AlwaysEncapsulate {
		return true
	}
	if field == "" {
		return false
	}
	if field == `\.` || strings.IndexRune(field, w.Comma) >= 0 || strings.IndexAny(field, "\"\r\n") >= 0 {
		return true
	}

	r1, _ := utf8.DecodeRuneInString(field)
	return unicode.IsSpace(r1)
}
