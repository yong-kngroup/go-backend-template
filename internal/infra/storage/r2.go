package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	appConfig "github.com/freeDog-wy/go-backend-template/internal/config"
	usecaseMedia "github.com/freeDog-wy/go-backend-template/internal/usecase/media"
)

type R2 struct {
	bucket, prefix string
	client         *s3.Client
	presigner      *s3.PresignClient
	ttl            time.Duration
}

func NewR2(ctx context.Context, cfg appConfig.R2Config) (*R2, error) {
	if strings.TrimSpace(cfg.AccountID) == "" || strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" || strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("R2 account_id, access_key_id, secret_access_key and bucket are required")
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("auto"), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")), awsconfig.WithBaseEndpoint(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) { o.UsePathStyle = true })
	ttl := time.Duration(cfg.PresignTTLMinutes) * time.Minute
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &R2{bucket: cfg.Bucket, prefix: strings.Trim(cfg.Prefix, "/"), client: client, presigner: s3.NewPresignClient(client), ttl: ttl}, nil
}

func (r *R2) HeadObject(ctx context.Context, key string) (*usecaseMedia.ObjectInfo, error) {
	out, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(r.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	return &usecaseMedia.ObjectInfo{ContentType: aws.ToString(out.ContentType), Size: aws.ToInt64(out.ContentLength)}, nil
}
func (r *R2) ObjectKey(name string) string {
	name = strings.TrimLeft(name, "/")
	if r.prefix == "" {
		return name
	}
	return r.prefix + "/" + name
}
func (r *R2) PresignUpload(ctx context.Context, key, contentType string) (*usecaseMedia.PresignedUpload, error) {
	request, err := r.presigner.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(r.bucket), Key: aws.String(key), ContentType: aws.String(contentType)}, s3.WithPresignExpires(r.ttl))
	if err != nil {
		return nil, err
	}
	return &usecaseMedia.PresignedUpload{URL: request.URL, Headers: map[string]string{"Content-Type": contentType}, ExpiresAt: time.Now().Add(r.ttl)}, nil
}
