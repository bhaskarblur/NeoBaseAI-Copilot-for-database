package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
)

// CompressData compresses data using gzip and returns base64 encoded string
func CompressData(data []byte) (string, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)

	_, err := gzipWriter.Write(data)
	if err != nil {
		return "", err
	}

	if err := gzipWriter.Close(); err != nil {
		return "", err
	}

	// Encode to base64 for safe storage
	compressed := base64.StdEncoding.EncodeToString(buf.Bytes())
	return compressed, nil
}

// DecompressData decompresses base64 encoded gzip data
func DecompressData(compressed string) ([]byte, error) {
	// Decode from base64
	decoded, err := base64.StdEncoding.DecodeString(compressed)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(decoded)
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	decompressed, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, err
	}

	return decompressed, nil
}
