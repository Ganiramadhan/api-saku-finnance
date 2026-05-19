package storage

import (
	"context"
	"mime/multipart"
	"time"
)

type Storage interface {
	Upload(ctx context.Context, file *multipart.FileHeader, folder string) (objectKey string, err error)
	UploadBytes(ctx context.Context, data []byte, contentType, folder, ext string) (objectKey string, err error)
	PresignedURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error)
	Move(ctx context.Context, srcKey, dstKey string) error
	Delete(ctx context.Context, objectKey string) error
}
