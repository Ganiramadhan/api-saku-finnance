package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type S3Config struct {
	AccessKey    string
	SecretKey    string
	Region       string
	Endpoint     string
	Bucket       string
	UsePathStyle bool
}

type S3Storage struct {
	client *s3.Client
	bucket string
}

func NewS3(cfg S3Config) (*S3Storage, error) {
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("s3: access key and secret key are required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3: bucket is required")
	}

	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
	}
	if cfg.Endpoint != "" {
		awsCfg.BaseEndpoint = aws.String(cfg.Endpoint)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})
	return &S3Storage{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3Storage) EnsureBucket(ctx context.Context) error {
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)}); err == nil {
		return nil
	}
	if _, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)}); err != nil {
		return fmt.Errorf("s3: bucket %q not found and could not be created: %w", s.bucket, err)
	}
	return nil
}

func (s *S3Storage) Upload(ctx context.Context, file *multipart.FileHeader, folder string) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer src.Close()

	body, err := io.ReadAll(src)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}

	ext := filepath.Ext(file.Filename)
	base := strings.TrimSuffix(filepath.Base(file.Filename), ext)
	if base == "" {
		base = "upload"
	}

	filename := fmt.Sprintf("%s-%s%s", base, uuid.New().String()[:8], ext)
	objectKey := fmt.Sprintf("%s/%s", strings.TrimSuffix(folder, "/"), filename)

	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}
	return objectKey, nil
}

func (s *S3Storage) PresignedURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	if objectKey == "" {
		return "", nil
	}
	presigner := s3.NewPresignClient(s.client)
	out, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}
	return out.URL, nil
}

func (s *S3Storage) Move(ctx context.Context, srcKey, dstKey string) error {
	if srcKey == "" || dstKey == "" {
		return fmt.Errorf("src and dst keys are required")
	}
	copySource := fmt.Sprintf("%s/%s", s.bucket, srcKey)
	if _, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(dstKey),
	}); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return s.Delete(ctx, srcKey)
}

func (s *S3Storage) Delete(ctx context.Context, objectKey string) error {
	if objectKey == "" {
		return nil
	}
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	}); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}
