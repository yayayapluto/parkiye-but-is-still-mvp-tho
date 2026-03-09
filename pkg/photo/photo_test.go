package photo_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"parkieee/pkg/config"
	"parkieee/pkg/photo"
)

func testS3Config(serverURL string) config.S3Config {
	return config.S3Config{
		Endpoint:      serverURL,
		Bucket:        "test-bucket",
		AccessKey:     "test-access-key",
		SecretKey:     "test-secret-key",
		Region:        "us-east-1",
		PublicBaseURL: serverURL + "/test-bucket",
	}
}

func newDoer(server *httptest.Server) photo.HTTPDoer {
	return photo.NewHTTPClientDoer(server.Client(), server.URL)
}

func TestSignedPutWith_Success(t *testing.T) {
	var receivedMethod, receivedPath, receivedContentType string
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedContentType = r.Header.Get("Content-Type")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := testS3Config(server.URL)
	data := []byte("fake image data")

	err := photo.SignedPutWith(newDoer(server), cfg, "entry/photo.jpg", data, "image/jpeg")

	require.NoError(t, err)
	assert.Equal(t, "PUT", receivedMethod)
	assert.Contains(t, receivedPath, "test-bucket")
	assert.Contains(t, receivedPath, "photo.jpg")
	assert.Equal(t, "image/jpeg", receivedContentType)
	assert.Equal(t, data, receivedBody)
}

func TestSignedPutWith_ServerReturns201(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	err := photo.SignedPutWith(newDoer(server), testS3Config(server.URL), "photo.jpg", []byte("data"), "image/jpeg")
	require.NoError(t, err)
}

func TestSignedPutWith_ServerReturns403_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("AccessDenied"))
	}))
	defer server.Close()

	err := photo.SignedPutWith(newDoer(server), testS3Config(server.URL), "photo.jpg", []byte("data"), "image/jpeg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestSignedPutWith_ServerReturns500_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("InternalError"))
	}))
	defer server.Close()

	err := photo.SignedPutWith(newDoer(server), testS3Config(server.URL), "photo.jpg", []byte("data"), "image/jpeg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestSignedPutWith_RequestHasAuthHeader(t *testing.T) {
	var authHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := photo.SignedPutWith(newDoer(server), testS3Config(server.URL), "photo.jpg", []byte("data"), "image/jpeg")

	require.NoError(t, err)
	assert.Contains(t, authHeader, "AWS4-HMAC-SHA256")
	assert.Contains(t, authHeader, "test-access-key")
	assert.Contains(t, authHeader, "Signature=")
}

func TestSignedPutWith_RequestHasAmzHeaders(t *testing.T) {
	var amzDate, amzContentSHA256 string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		amzDate = r.Header.Get("x-amz-date")
		amzContentSHA256 = r.Header.Get("x-amz-content-sha256")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := photo.SignedPutWith(newDoer(server), testS3Config(server.URL), "photo.jpg", []byte("data"), "image/jpeg")

	require.NoError(t, err)
	assert.NotEmpty(t, amzDate, "x-amz-date header must be set")
	assert.NotEmpty(t, amzContentSHA256, "x-amz-content-sha256 header must be set")
}

func TestSignedPutWith_ConnectionRefused_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close() // tutup langsung — simulasi server mati

	err := photo.SignedPutWith(newDoer(server), testS3Config(server.URL), "photo.jpg", []byte("data"), "image/jpeg")
	require.Error(t, err)
}

func TestEnsureBucketPolicyWith_Success(t *testing.T) {
	var receivedMethod, receivedPath string
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.RequestURI()
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := photo.EnsureBucketPolicyWith(newDoer(server), testS3Config(server.URL))

	require.NoError(t, err)
	assert.Equal(t, "PUT", receivedMethod)
	assert.Contains(t, receivedPath, "policy")
	assert.Contains(t, string(receivedBody), "s3:GetObject")
	assert.Contains(t, string(receivedBody), "test-bucket")
}

func TestEnsureBucketPolicyWith_Returns200_NoError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := photo.EnsureBucketPolicyWith(newDoer(server), testS3Config(server.URL))
	require.NoError(t, err)
}

func TestEnsureBucketPolicyWith_Returns400_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("MalformedPolicy"))
	}))
	defer server.Close()

	err := photo.EnsureBucketPolicyWith(newDoer(server), testS3Config(server.URL))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestEnsureBucketPolicyWith_EmptyConfig_Skipped(t *testing.T) {
	// Kalau Bucket atau Endpoint kosong, tidak boleh ada request keluar
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	err := photo.EnsureBucketPolicyWith(newDoer(server), config.S3Config{})
	require.NoError(t, err)
	assert.False(t, called, "no request should be made when config is empty")
}
