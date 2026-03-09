package photo

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"parkieee/pkg/config"
)

// Save uploads the file to S3-compatible storage using raw AWS Signature V4
// (bypasses SDK quirks with Ceph-based providers like NevaObjects).
// Returns:
//   - publicURL : full HTTPS URL to the object (for serving to clients)
//   - ocrPath   : same URL, used by the OCR service to fetch the image
func Save(file *multipart.FileHeader, prefix string, s3cfg config.S3Config) (publicURL string, ocrPath string, err error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		ext = ".jpg"
	}

	filename := fmt.Sprintf("%s_%s_%d%s",
		prefix,
		uuid.New().String()[:8],
		time.Now().UnixMilli(),
		ext,
	)

	src, err := file.Open()
	if err != nil {
		return "", "", fmt.Errorf("open uploaded file: %w", err)
	}
	defer func(src multipart.File) {
		err := src.Close()
		if err != nil {

		}
	}(src)

	body, err := io.ReadAll(src)
	if err != nil {
		return "", "", fmt.Errorf("read file: %w", err)
	}

	contentType := "image/jpeg"
	if ext == ".png" {
		contentType = "image/png"
	}

	if err := signedPut(s3cfg, filename, body, contentType); err != nil {
		return "", "", fmt.Errorf("upload to s3 (bucket=%s endpoint=%s): %w", s3cfg.Bucket, s3cfg.Endpoint, err)
	}

	publicURL = strings.TrimRight(s3cfg.PublicBaseURL, "/") + "/" + filename
	ocrPath = publicURL

	return publicURL, ocrPath, nil
}

// EnsureBucketPolicy sets the bucket to public-read so uploaded files are accessible.
// Safe to call on every startup — returns nil if policy already exists (204).
func EnsureBucketPolicy(cfg config.S3Config) error {
	return EnsureBucketPolicyWith(httpClient, cfg)
}

// EnsureBucketPolicyWith is the testable version of EnsureBucketPolicy.
func EnsureBucketPolicyWith(client HTTPDoer, cfg config.S3Config) error {
	if cfg.Bucket == "" || cfg.Endpoint == "" {
		return nil
	}

	policy := fmt.Sprintf(
		`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`,
		cfg.Bucket,
	)

	rawURL := fmt.Sprintf("%s/%s/?policy", strings.TrimRight(cfg.Endpoint, "/"), cfg.Bucket)
	status, body, err := doSignedRequestWith(client, cfg, "PUT", rawURL, "application/json", []byte(policy))
	if err != nil {
		return fmt.Errorf("bucket policy request: %w", err)
	}
	if status != 200 && status != 204 {
		return fmt.Errorf("bucket policy returned %d: %s", status, body)
	}
	return nil
}

// signedPut is a convenience wrapper around doSignedRequest for simple PUTs without query strings.
func signedPut(cfg config.S3Config, key string, data []byte, contentType string) error {
	return SignedPutWith(httpClient, cfg, key, data, contentType)
}

// SignedPutWith is the testable version of signedPut — accepts an HTTPDoer so tests can
// substitute a fake server without touching the package-level httpClient.
func SignedPutWith(client HTTPDoer, cfg config.S3Config, key string, data []byte, contentType string) error {
	rawURL := fmt.Sprintf("%s/%s/%s", strings.TrimRight(cfg.Endpoint, "/"), cfg.Bucket, key)
	status, respBody, err := doSignedRequestWith(client, cfg, "PUT", rawURL, contentType, data)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("s3 returned %d: %s", status, respBody)
	}
	return nil
}

// HTTPDoer is the minimal interface required to make HTTP requests.
// The default implementation uses httpClient; tests can substitute a mock or httptest server.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// NewHTTPClientDoer wraps an *http.Client so that all requests are rewritten to
// targetBaseURL. Used in tests to redirect S3 calls to an httptest.Server.
func NewHTTPClientDoer(c *http.Client, targetBaseURL string) HTTPDoer {
	return &rewritingDoer{client: c, base: strings.TrimRight(targetBaseURL, "/")}
}

type rewritingDoer struct {
	client *http.Client
	base   string
}

func (d *rewritingDoer) Do(req *http.Request) (*http.Response, error) {
	// Keep path + query, only replace scheme+host with the test server base.
	req.URL.Scheme = ""
	req.URL.Host = ""
	newURL := d.base + req.URL.RequestURI()
	parsed, err := url.Parse(newURL)
	if err != nil {
		return nil, err
	}
	req.URL = parsed
	req.Host = parsed.Host
	return d.client.Do(req)
}

// httpClient is a shared HTTP client with a timeout to prevent goroutine leaks
// when S3 endpoints are slow or unresponsive.
var httpClient HTTPDoer = &http.Client{Timeout: 30 * time.Second}

// doSignedRequest performs an AWS Signature V4 signed HTTP request.
// Handles both plain PUT uploads and requests with query strings (e.g. ?policy).
func doSignedRequest(cfg config.S3Config, method, rawURL, contentType string, body []byte) (int, string, error) {
	return doSignedRequestWith(httpClient, cfg, method, rawURL, contentType, body)
}

func doSignedRequestWith(client HTTPDoer, cfg config.S3Config, method, rawURL, contentType string, body []byte) (int, string, error) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0, "", err
	}

	host := strings.TrimPrefix(cfg.Endpoint, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimRight(host, "/")

	canonicalURI := parsed.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	// Canonical query string must be sorted for signature stability.
	qp := parsed.Query()
	qKeys := make([]string, 0, len(qp))
	for k := range qp {
		qKeys = append(qKeys, k)
	}
	sort.Strings(qKeys)
	var cqs strings.Builder
	for i, k := range qKeys {
		if i > 0 {
			cqs.WriteString("&")
		}
		vals := qp[k]
		if len(vals) == 0 || (len(vals) == 1 && vals[0] == "") {
			cqs.WriteString(url.QueryEscape(k) + "=")
		} else {
			sort.Strings(vals)
			for j, v := range vals {
				if j > 0 {
					cqs.WriteString("&")
				}
				cqs.WriteString(url.QueryEscape(k) + "=" + url.QueryEscape(v))
			}
		}
	}

	payloadHash := sha256hex(body)

	hdrs := map[string]string{
		"content-type":         contentType,
		"host":                 host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	hdrKeys := make([]string, 0, len(hdrs))
	for k := range hdrs {
		hdrKeys = append(hdrKeys, k)
	}
	sort.Strings(hdrKeys)

	var canonHdrs, signedHdrs strings.Builder
	for i, k := range hdrKeys {
		canonHdrs.WriteString(k + ":" + hdrs[k] + "\n")
		if i > 0 {
			signedHdrs.WriteString(";")
		}
		signedHdrs.WriteString(k)
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	canonicalReq := strings.Join([]string{
		method, canonicalURI, cqs.String(),
		canonHdrs.String(), signedHdrs.String(), payloadHash,
	}, "\n")

	credScope := dateStamp + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzDate, credScope, sha256hex([]byte(canonicalReq)),
	}, "\n")

	sigKey := signingKey(cfg.SecretKey, dateStamp, region, "s3")
	sig := hex.EncodeToString(hmacSHA256(sigKey, []byte(stringToSign)))
	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		cfg.AccessKey, credScope, signedHdrs.String(), sig,
	)

	req, err := http.NewRequest(method, rawURL, bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("Authorization", authHeader)
	req.ContentLength = int64(len(body))

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func signingKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}
