package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrNotFound = errors.New("object not found")

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type Store struct {
	client *minio.Client
	bucket string
}

func New(ctx context.Context, cfg Config) (*Store, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("configure minio: %w", err)
	}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check minio bucket: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("minio bucket %q does not exist", cfg.Bucket)
	}
	return &Store{client: client, bucket: cfg.Bucket}, nil
}

func (s *Store) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, int64, string, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, "", fmt.Errorf("get object: %w", err)
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		if isNotFound(err) {
			return nil, 0, "", ErrNotFound
		}
		return nil, 0, "", fmt.Errorf("stat object: %w", err)
	}
	return object, info.Size, info.ContentType, nil
}

func (s *Store) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("remove object: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	response := minio.ToErrorResponse(err)
	return response.Code == "NoSuchKey" || response.Code == "NoSuchBucket" || response.StatusCode == 404
}
