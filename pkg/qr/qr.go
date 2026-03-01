package qr

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	goqrcode "github.com/skip2/go-qrcode"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"parkieee/pkg/logger"
)

const (
	storageDir = "storage/qr"
	urlPrefix  = "/storage/qr"
	ticketSize = 512 // canvas 512x512 (1:1)
	marginX    = 20
	fontSize   = 12
	titleSize  = 14
)

var (
	colBlack = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	colWhite = color.RGBA{R: 255, G: 255, B: 255, A: 255}
)

// TicketData holds all the info needed to generate a parking ticket image.
type TicketData struct {
	TransactionCode string
	PlaceName       string // from PLACE_NAME env
	EntryAt         time.Time
	QRText          string
}

// SaveTicket generates a 1:1 thermal-printer-style parking ticket PNG,
// saves it to storage/qr/<obfuscated>.png, and returns the public URL.
// The filename is HMAC-SHA256(secret, transactionCode)[:16] so it is
// unpredictable while the transaction code in the DB stays unchanged.
func SaveTicket(ctx context.Context, log logger.Logger, baseURL, secret string, data TicketData) (string, error) {
	log.Debug(ctx, "qr: generating ticket",
		"transaction_code", data.TransactionCode,
		"place_name", data.PlaceName,
		"entry_at", data.EntryAt,
		"qr_text", data.QRText,
	)

	if err := os.MkdirAll(storageDir, 0755); err != nil {
		log.Error(ctx, "qr: failed to create storage dir",
			"dir", storageDir,
			"error", err,
		)
		return "", fmt.Errorf("create qr storage dir: %w", err)
	}

	// ── 1. Load fonts ────────────────────────────────────────────────────────
	fBoldTitle, err := parseFace(gomonobold.TTF, titleSize)
	if err != nil {
		log.Error(ctx, "qr: failed to parse bold title font", "size", titleSize, "error", err)
		return "", err
	}
	fReg, err := parseFace(gomono.TTF, fontSize)
	if err != nil {
		log.Error(ctx, "qr: failed to parse regular font", "size", fontSize, "error", err)
		return "", err
	}

	// ── 2. Plain QR text: strip "PARKIEEE-" prefix only, keep dashes ─────────
	plainQR := strings.TrimPrefix(data.QRText, "PARKIEEE-")

	// ── 3. Layout constants ───────────────────────────────────────────────────
	lh := lineH(fReg)
	lhTitle := lineH(fBoldTitle)
	gap := 8

	infoRows := []struct{ label, value string }{
		{"Ticket No :", data.TransactionCode},
		{"Date      :", data.EntryAt.Format("02 Jan 2006")},
		{"Time      :", data.EntryAt.Format("15:04:05")},
	}

	// ── 4. Compute QR size from remaining space ───────────────────────────────
	marginY := 18

	textH := 0
	textH += lhTitle + gap*3            // TANDA MASUK
	textH += len(infoRows)*(lh+4) + gap // info rows
	// [QR here]
	textH += gap      // gap after QR
	textH += lh + gap // plain QR text
	textH += lh + 3   // pembayaran
	textH += lh       // terima kasih

	qrSize := ticketSize - textH - 2*marginY
	if qrSize < 80 {
		qrSize = 80
	}
	maxQR := ticketSize - 2*marginX
	if qrSize > maxQR {
		qrSize = maxQR
	}

	log.Debug(ctx, "qr: layout calculated",
		"text_height", textH,
		"qr_size", qrSize,
		"canvas", ticketSize,
	)

	// ── 5. Generate QR code (always square) ───────────────────────────────────
	qrCode, err := goqrcode.New(data.QRText, goqrcode.Medium)
	if err != nil {
		log.Error(ctx, "qr: failed to generate qr code",
			"qr_text", data.QRText,
			"error", err,
		)
		return "", fmt.Errorf("generate qr code: %w", err)
	}
	qrImg := qrCode.Image(qrSize)

	// ── 6. Create 1:1 white canvas ────────────────────────────────────────────
	img := image.NewRGBA(image.Rect(0, 0, ticketSize, ticketSize))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: colWhite}, image.Point{}, draw.Src)

	y := marginY

	// ── 7. TANDA MASUK ────────────────────────────────────────────────────────
	drawCentered(img, fBoldTitle, y, "TANDA MASUK")
	y += lhTitle + gap*3

	// ── 8. Info rows ─────────────────────────────────────────────────────────
	for _, row := range infoRows {
		drawText(img, fReg, marginX, y, row.label+" "+row.value)
		y += lh + 4
	}
	y += gap

	// ── 10. QR code (centered, 1:1) ───────────────────────────────────────────
	qrX := (ticketSize - qrSize) / 2
	draw.Draw(img, image.Rect(qrX, y, qrX+qrSize, y+qrSize), qrImg, image.Point{}, draw.Src)
	y += qrSize + gap

	// ── 11. Plain QR text ─────────────────────────────────────────────────────
	drawCentered(img, fReg, y, plainQR)
	y += lh + gap

	// ── 12. Footer ────────────────────────────────────────────────────────────
	drawCentered(img, fReg, y, "Pembayaran: QRIS / Tunai")
	y += lh + 3
	drawCentered(img, fReg, y, "Terima kasih")

	// ── 13. Save PNG ──────────────────────────────────────────────────────────
	filename := obfuscateFilename(secret, data.TransactionCode)
	dest := filepath.Join(storageDir, filename+".png")
	f, err := os.Create(dest)
	if err != nil {
		log.Error(ctx, "qr: failed to create ticket file",
			"dest", dest,
			"transaction_code", data.TransactionCode,
			"error", err,
		)
		return "", fmt.Errorf("create ticket file: %w", err)
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)

	if err := png.Encode(f, img); err != nil {
		log.Error(ctx, "qr: failed to encode ticket png",
			"dest", dest,
			"transaction_code", data.TransactionCode,
			"error", err,
		)
		return "", fmt.Errorf("encode ticket png: %w", err)
	}

	url := baseURL + urlPrefix + "/" + filename + ".png"
	log.Info(ctx, "qr: ticket saved",
		"transaction_code", data.TransactionCode,
		"filename", filename,
		"dest", dest,
		"url", url,
	)

	return url, nil
}

// SavePNG is the original plain QR code generator, kept for backward compatibility.
func SavePNG(ctx context.Context, log logger.Logger, baseURL, filename, content string) (string, error) {
	log.Debug(ctx, "qr: generating plain qr png",
		"filename", filename,
		"content", content,
	)

	if err := os.MkdirAll(storageDir, 0755); err != nil {
		log.Error(ctx, "qr: failed to create storage dir", "dir", storageDir, "error", err)
		return "", fmt.Errorf("create qr storage dir: %w", err)
	}

	dest := filepath.Join(storageDir, filename+".png")
	if err := goqrcode.WriteFile(content, goqrcode.Medium, 256, dest); err != nil {
		log.Error(ctx, "qr: failed to write qr png",
			"dest", dest,
			"filename", filename,
			"error", err,
		)
		return "", fmt.Errorf("write qr png: %w", err)
	}

	url := baseURL + urlPrefix + "/" + filename + ".png"
	log.Info(ctx, "qr: plain png saved", "dest", dest, "url", url)
	return url, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// obfuscateFilename returns HMAC-SHA256(secret, code)[:16] as the filename.
// Same secret+code always produces the same filename (deterministic),
// but impossible to reverse-engineer the transaction code from the URL.
func obfuscateFilename(secret, code string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

func parseFace(ttfBytes []byte, size float64) (font.Face, error) {
	tt, err := opentype.Parse(ttfBytes)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	face, err := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    size,
		DPI:     96,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("new face: %w", err)
	}
	return face, nil
}

func lineH(face font.Face) int {
	m := face.Metrics()
	return (m.Ascent + m.Descent).Ceil() + 3
}

func drawCentered(img *image.RGBA, face font.Face, y int, text string) {
	adv := font.MeasureString(face, text)
	x := (ticketSize - adv.Ceil()) / 2
	if x < marginX {
		x = marginX
	}
	drawText(img, face, x, y, text)
}

func drawText(img *image.RGBA, face font.Face, x, y int, text string) {
	asc := face.Metrics().Ascent.Ceil()
	(&font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(colBlack),
		Face: face,
		Dot:  fixed.P(x, y+asc),
	}).DrawString(text)
}
