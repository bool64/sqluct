package sqluct

import "strings"

// SplitStatements splits a string in multiple SQL statements separated by semicolon (';').
//
// Semicolons in comments and string literals are not treated as separators.
func SplitStatements(s string) []string {
	var (
		quoteChars  = [...]int32{'"', '\'', '`'}
		quoteOpened int32
	)

	prevStart := 0
	prevQuot := false

	// -- Line comment.
	lineCommentStarted := false
	prevDash := false

	// /* Block comment */
	blockCommentStarted := false
	prevSlash := false
	prevAsterisk := false

	var res []string

	for i, c := range s {
		if blockCommentStarted && c != '*' && c != '/' {
			continue
		}

		if lineCommentStarted {
			if c == '\n' {
				lineCommentStarted = false

				continue
			}

			continue
		}

		if quoteOpened != 0 {
			// This may be a closing quote or an escaped quot if it is immediately followed by another same quot.
			if c == quoteOpened {
				if prevQuot {
					prevQuot = false

					continue
				}

				prevQuot = true

				continue
			}

			if prevQuot {
				prevQuot = false
				quoteOpened = 0
			}
		}

		// quoteOpened is 0
		for _, q := range quoteChars {
			if c == q {
				quoteOpened = q
			}
		}

		if quoteOpened != 0 {
			continue
		}

		// Might be a line comment.
		if c == '-' {
			if prevDash {
				prevDash = false
				lineCommentStarted = true

				continue
			}

			prevDash = true
		} else {
			prevDash = false
		}

		if c == '/' {
			if prevAsterisk && blockCommentStarted {
				blockCommentStarted = false
				prevAsterisk = false

				continue
			}

			prevSlash = true

			continue
		}

		if c == '*' {
			if prevSlash && !blockCommentStarted {
				blockCommentStarted = true
				prevSlash = false

				continue
			}

			prevAsterisk = true

			continue
		}

		prevSlash = false
		prevAsterisk = false

		// Not in an enquoted string, so that's a statement separator.
		if c == ';' {
			st := strings.TrimSpace(s[prevStart:i])
			if len(st) > 0 {
				res = append(res, st)
			}

			prevStart = i + 1
		}
	}

	st := strings.TrimSpace(s[prevStart:])
	if len(st) > 0 {
		res = append(res, st)
	}

	return res
}
