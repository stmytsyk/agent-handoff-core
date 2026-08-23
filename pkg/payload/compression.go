package payload

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

const CompressionZstd = "zstd"

type Envelope struct {
	SchemaVersion string `json:"schema_version"`
	Encoding      string `json:"encoding"`
	Compression   string `json:"compression"`
	Payload       string `json:"payload"`
}

func CompressZstd(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	encoder, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, err
	}
	if _, err := encoder.Write(data); err != nil {
		encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecompressZstd(data []byte) ([]byte, error) {
	decoder, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer decoder.Close()
	return decoder.DecodeAll(data, nil)
}

func EncodeEnvelope(data []byte) (Envelope, error) {
	compressed, err := CompressZstd(data)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		SchemaVersion: "ahp-envelope/v1",
		Encoding:      "base64",
		Compression:   CompressionZstd,
		Payload:       base64.StdEncoding.EncodeToString(compressed),
	}, nil
}

func DecodeEnvelope(envelope Envelope) ([]byte, error) {
	if envelope.Encoding != "base64" {
		return nil, fmt.Errorf("unsupported envelope encoding %q", envelope.Encoding)
	}
	if envelope.Compression != CompressionZstd {
		return nil, fmt.Errorf("unsupported envelope compression %q", envelope.Compression)
	}
	compressed, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return nil, err
	}
	return DecompressZstd(compressed)
}

func (b Builder) CompressedEnvelope(ctx context.Context, opts Options) (Envelope, error) {
	data, err := b.JSON(ctx, opts)
	if err != nil {
		return Envelope{}, err
	}
	return EncodeEnvelope(data)
}
