package photo

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Save writes the uploaded file to storageDir and returns:
//   - publicURL : relative URL for serving the file (e.g. /storage/photos/entry_xxx.jpg)
//   - ocrPath   : path as seen by the Python OCR container
//
// ocrPrefix should be the container-side mount path (e.g. /mnt/storage/photos).
// It must come from the parsed config struct, not os.Getenv, to avoid Git Bash
// path translation that corrupts POSIX paths exported from the Makefile.
func Save(file *multipart.FileHeader, prefix, storageDir, ocrPrefix string) (publicURL string, ocrPath string, err error) {
	if err = os.MkdirAll(storageDir, 0755); err != nil {
		return "", "", fmt.Errorf("create photo dir: %w", err)
	}

	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	ext = strings.ToLower(ext)

	filename := fmt.Sprintf("%s_%s_%d%s",
		prefix,
		uuid.New().String()[:8],
		time.Now().UnixMilli(),
		ext,
	)

	localPath := filepath.Join(storageDir, filename)

	src, err := file.Open()
	if err != nil {
		return "", "", fmt.Errorf("open uploaded file: %w", err)
	}
	defer func(src multipart.File) {
		err := src.Close()
		if err != nil {

		}
	}(src)

	dst, err := os.Create(localPath)
	if err != nil {
		return "", "", fmt.Errorf("create destination file: %w", err)
	}
	defer func(dst *os.File) {
		err := dst.Close()
		if err != nil {

		}
	}(dst)

	if _, err = io.Copy(dst, src); err != nil {
		return "", "", fmt.Errorf("write file: %w", err)
	}

	publicURL = "/storage/photos/" + filename

	if ocrPrefix != "" {
		ocrPath = strings.TrimRight(ocrPrefix, "/") + "/" + filename
	} else {
		ocrPath = filepath.ToSlash(localPath)
	}

	return publicURL, ocrPath, nil
}
