// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// malformedSubmission is shown for requests an ordinary form never sends.
// Handlers log the specific problem for the operator.
const malformedSubmission = "Something went wrong with this submission. Try again."

// sentence turns a Go error string into display text: a capital first letter
// and a closing full stop.
func sentence(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return message
	}
	first, size := utf8.DecodeRuneInString(message)
	message = string(unicode.ToUpper(first)) + message[size:]
	if !strings.HasSuffix(message, ".") && !strings.HasSuffix(message, "?") && !strings.HasSuffix(message, "!") {
		message += "."
	}
	return message
}

// logValue bounds client-supplied text before it reaches the server log.
func logValue(value string) string {
	const limit = 64
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
