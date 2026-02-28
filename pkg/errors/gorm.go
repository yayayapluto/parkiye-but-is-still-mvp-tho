package errors

import (
	goerrors "errors"
	"strings"

	"gorm.io/gorm"
)

// FromDB converts a GORM error into an AppError.
// notFoundMsg dipakai sebagai message kalau errornya ErrRecordNotFound —
// tiap repo bisa kasih pesan yang lebih spesifik, e.g. "user not found".
func FromDB(err error, notFoundMsg string) error {
	if err == nil {
		return nil
	}
	// GORM v2 docs: gunakan errors.Is, bukan == untuk ErrRecordNotFound
	if goerrors.Is(err, gorm.ErrRecordNotFound) {
		return New(ErrNotFound, notFoundMsg)
	}
	if strings.Contains(err.Error(), "duplicate key") {
		return New(ErrDuplicate, "data already exists")
	}
	return Wrap(err, ErrDatabaseError, "database error")
}
