package archive

import (
	"errors"
	"strings"

	"github.com/nwaples/rardecode/v2"
	"github.com/unxed/sevenzip"
)

// IsPasswordError reports whether an archive operation failed because the
// archive or one of its entries requires a password, or because the supplied
// password was rejected. It is intentionally format-independent so callers
// can decide whether to ask their user for credentials without depending on
// the concrete archive implementation.
func IsPasswordError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, rardecode.ErrArchiveEncrypted) || errors.Is(err, rardecode.ErrArchivedFileEncrypted) {
		return true
	}
	var readErr sevenzip.ReadError
	if errors.As(err, &readErr) && readErr.Encrypted {
		return true
	}
	var readErrPtr *sevenzip.ReadError
	if errors.As(err, &readErrPtr) && readErrPtr != nil && readErrPtr.Encrypted {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "password") || strings.Contains(message, "encrypted")
}
