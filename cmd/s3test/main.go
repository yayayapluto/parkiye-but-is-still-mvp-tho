package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	endpoint  = "https://s3.nevaobjects.id"
	bucket    = "parkieee"
	accessKey = "5ID4GIJYHP7UPOEPCZEZ"
	secretKey = "UktAA0gM7GB6O30dnkcwf3z40P1AtfILI0CCl7dI"
	region    = "us-east-1"
	s3host    = "s3.nevaobjects.id"
)

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func signingKey(secret, date, reg, service string) []byte {
	k := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	k = hmacSHA256(k, []byte(reg))
	k = hmacSHA256(k, []byte(service))
	return hmacSHA256(k, []byte("aws4_request"))
}

func doRequest(method, rawURL, contentType string, body []byte) (int, string) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	parsed, _ := url.Parse(rawURL)

	canonicalURI := parsed.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	// Build canonical query string — keys sorted, empty values kept as key=
	queryParams := parsed.Query()
	queryKeys := make([]string, 0, len(queryParams))
	for k := range queryParams {
		queryKeys = append(queryKeys, k)
	}
	sort.Strings(queryKeys)
	var cqs strings.Builder
	for i, k := range queryKeys {
		if i > 0 {
			cqs.WriteString("&")
		}
		vals := queryParams[k]
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

	payloadHash := sha256Hex(body)

	hdrs := map[string]string{
		"content-type":         contentType,
		"host":                 s3host,
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

	canonicalReq := strings.Join([]string{
		method,
		canonicalURI,
		cqs.String(),
		canonHdrs.String(),
		signedHdrs.String(),
		payloadHash,
	}, "\n")

	credScope := dateStamp + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credScope,
		sha256Hex([]byte(canonicalReq)),
	}, "\n")

	sig := hex.EncodeToString(hmacSHA256(signingKey(secretKey, dateStamp, region, "s3"), []byte(stringToSign)))
	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credScope, signedHdrs.String(), sig,
	)

	req, _ := http.NewRequestWithContext(context.Background(), method, rawURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("Authorization", authHeader)
	req.ContentLength = int64(len(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody)
}

func main() {
	// ── 1. Set bucket policy: public read ────────────────────────────────────
	fmt.Println("=== Setting bucket policy (public-read) ===")
	policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::parkieee/*"]}]}`
	policyURL := fmt.Sprintf("%s/%s/?policy", endpoint, bucket)
	status, body := doRequest("PUT", policyURL, "application/json", []byte(policy))
	fmt.Printf("Status: %d\nBody: %s\n", status, body)
	if status == 200 || status == 204 {
		fmt.Println("✅ Bucket policy set!")
	} else {
		fmt.Println("⚠️  Policy PUT gagal")
	}

	fmt.Println()

	// ── 2. Upload test file ───────────────────────────────────────────────────
	fmt.Println("=== Uploading test file ===")
	testBody := []byte("hello from parkieee s3test")
	uploadURL := fmt.Sprintf("%s/%s/test-public.txt", endpoint, bucket)
	status, body = doRequest("PUT", uploadURL, "text/plain", testBody)
	fmt.Printf("Status: %d\nBody: %s\n", status, body)
	if status != 200 && status != 201 {
		fmt.Fprintln(os.Stderr, "❌ Upload gagal")
		os.Exit(1)
	}
	fmt.Println("✅ Upload sukses!")

	fmt.Println()

	// ── 3. Verify public access ───────────────────────────────────────────────
	fmt.Println("=== Verifying public access ===")
	pubURL := "https://parkieee.s3.nevaobjects.id/test-public.txt"
	resp, err := http.Get(pubURL)
	if err != nil {
		fmt.Println("❌ Error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	if resp.StatusCode == 200 {
		fmt.Println("✅ File publicly accessible! OCR akan bisa fetch foto.")
	} else {
		fmt.Println("❌ Still AccessDenied:", string(respBody))
		fmt.Println()
		fmt.Println("NevaObjects tidak support bucket policy via API.")
		fmt.Println("Solusi: upload tiap file dengan x-amz-acl: public-read header.")
	}
}
