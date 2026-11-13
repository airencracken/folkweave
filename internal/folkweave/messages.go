// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/airencracken/comfylib/autobackup"
)

// malformedSubmission is shown for requests an ordinary form never sends.
// Handlers log the specific problem for the operator.
const malformedSubmission = "Something went wrong with this submission. Try again."

// backupProblem replaces raw backup errors on owner pages. The backup runner
// logs the underlying error, which can include paths and system details.
const backupProblem = "The last backup did not finish. The server log has the details."

func ownerBackupStatus(status autobackup.Status) autobackup.Status {
	if status.LastError != "" {
		status.LastError = backupProblem
	}
	return status
}

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
