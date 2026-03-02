package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"parkieee/pkg/middleware"
	"parkieee/pkg/response"
)

const (
	photosDir   = "/mnt/storage/photos"
	maxFileSize = 10 << 20 // 10 MB
)

var allowedExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

// RegisterRoutes mounts POST /storage/photos under the given router group.
func RegisterRoutes(router fiber.Router, auth middleware.TokenValidator) {
	h := &handler{}
	g := router.Group("/storage", middleware.Auth(auth))
	g.Post("/photos", h.uploadPhoto)
}

type handler struct{}

// uploadPhoto godoc
//
//	POST /api/v1/storage/photos
//	Content-Type: multipart/form-data
//	Field: photo (required, jpg/jpeg/png, max 10MB)
//
//	Response 201:
//	  {
//	    "path": "/mnt/storage/photos/20260302_<uuid>.jpg",  ← kirim ke entry_photo_path
//	    "url":  "/storage/photos/20260302_<uuid>.jpg"       ← akses publik via Go API
//	  }
func (h *handler) uploadPhoto(c *fiber.Ctx) error {
	file, err := c.FormFile("photo")
	if err != nil {
		return response.BadRequest(c, "field 'photo' wajib diisi (multipart/form-data)", nil)
	}

	if file.Size > maxFileSize {
		return response.BadRequest(c, fmt.Sprintf("ukuran file maksimal %d MB", maxFileSize>>20), nil)
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedExts[ext] {
		return response.BadRequest(c, "format file tidak didukung, gunakan jpg atau png", nil)
	}

	// Filename: YYYYMMDD_<uuid><ext>  → unik, sortable by date
	filename := fmt.Sprintf("%s_%s%s",
		time.Now().Format("20060102"),
		uuid.New().String(),
		ext,
	)

	savePath := filepath.Join(photosDir, filename)
	if err := c.SaveFile(file, savePath); err != nil {
		return fmt.Errorf("gagal menyimpan foto: %w", err)
	}

	return response.Created(c, "foto berhasil diupload", fiber.Map{
		"path": savePath,
		"url":  "/storage/photos/" + filename,
	})
}
