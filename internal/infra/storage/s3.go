package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	appConfig "github.com/freeDog-wy/go-backend-template/internal/config"
	domainMedia "github.com/freeDog-wy/go-backend-template/internal/domain/media"
)

type S3 struct {
	bucket, prefix string
	publicBaseURL  string
	client         *s3.Client
	presigner      *s3.PresignClient
	ttl            time.Duration
}

var _ domainMedia.Storage = (*S3)(nil)

func NewS3(ctx context.Context, cfg appConfig.S3Config) (*S3, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" || strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" || strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("S3 endpoint, access_key_id, secret_access_key and bucket are required")
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "auto"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		awsconfig.WithBaseEndpoint(strings.TrimRight(cfg.Endpoint, "/")),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) { o.UsePathStyle = cfg.UsePathStyle })
	ttl := time.Duration(cfg.PresignTTLMinutes) * time.Minute
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &S3{bucket: cfg.Bucket, prefix: strings.Trim(cfg.Prefix, "/"), publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"), client: client, presigner: s3.NewPresignClient(client), ttl: ttl}, nil
}

func (s *S3) HeadObject(ctx context.Context, key string) (*domainMedia.ObjectInfo, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	return &domainMedia.ObjectInfo{ContentType: aws.ToString(out.ContentType), Size: aws.ToInt64(out.ContentLength)}, nil
}

func (s *S3) OpenObject(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (s *S3) ObjectKey(name string) string {
	name = strings.TrimLeft(name, "/")
	if s.prefix == "" {
		return name
	}
	return s.prefix + "/" + name
}

func (s *S3) PublicURL(key string) string {
	if s.publicBaseURL == "" {
		return ""
	}
	return s.publicBaseURL + "/" + strings.TrimLeft(key, "/")
}

func (s *S3) PresignUpload(ctx context.Context, key, contentType string) (*domainMedia.PresignedUpload, error) {
	request, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(contentType)}, s3.WithPresignExpires(s.ttl))
	if err != nil {
		return nil, err
	}
	return &domainMedia.PresignedUpload{URL: request.URL, Headers: map[string]string{"Content-Type": contentType}, ExpiresAt: time.Now().Add(s.ttl)}, nil
}
