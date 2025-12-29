package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// R2Service R2 存储服务
type R2Service struct {
	client     *s3.Client
	bucketName string
	db         *sql.DB
}

// R2Config R2 配置
type R2Config struct {
	Endpoint        string `json:"endpoint"`          // 完整端点 URL，如 https://xxx.r2.cloudflarestorage.com
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	BucketName      string `json:"bucket_name"`
}

// NewR2Service 创建 R2 服务实例
func NewR2Service(db *sql.DB) (*R2Service, error) {
	log.Println("[R2] 正在初始化 R2 服务...")

	// 从数据库加载配置
	r2Config, err := LoadR2Config(db)
	if err != nil {
		log.Printf("[R2] 加载配置失败: %v", err)
		return nil, fmt.Errorf("加载 R2 配置失败: %w", err)
	}

	log.Printf("[R2] 配置加载成功: endpoint=%s, bucket=%s", r2Config.Endpoint, r2Config.BucketName)

	// 创建 S3 客户端
	client, err := createS3Client(r2Config)
	if err != nil {
		log.Printf("[R2] 创建客户端失败: %v", err)
		return nil, fmt.Errorf("创建 S3 客户端失败: %w", err)
	}

	log.Println("[R2] R2 服务初始化成功")

	return &R2Service{
		client:     client,
		bucketName: r2Config.BucketName,
		db:         db,
	}, nil
}

// NewR2ServiceWithConfig 使用指定配置创建 R2 服务（用于测试连接）
func NewR2ServiceWithConfig(r2Config *R2Config) (*R2Service, error) {
	log.Printf("[R2] 使用配置创建服务: endpoint=%s, bucket=%s", r2Config.Endpoint, r2Config.BucketName)

	client, err := createS3Client(r2Config)
	if err != nil {
		log.Printf("[R2] 创建客户端失败: %v", err)
		return nil, err
	}

	return &R2Service{
		client:     client,
		bucketName: r2Config.BucketName,
	}, nil
}

// LoadR2Config 从数据库加载 R2 配置
func LoadR2Config(db *sql.DB) (*R2Config, error) {
	var endpoint, accessKeyID, secretAccessKey, bucketName string

	err := db.QueryRow("SELECT value FROM system_config WHERE key = 'r2_endpoint'").Scan(&endpoint)
	if err != nil {
		return nil, fmt.Errorf("未找到 R2 配置")
	}

	db.QueryRow("SELECT value FROM system_config WHERE key = 'r2_access_key_id'").Scan(&accessKeyID)
	db.QueryRow("SELECT value FROM system_config WHERE key = 'r2_secret_access_key'").Scan(&secretAccessKey)
	db.QueryRow("SELECT value FROM system_config WHERE key = 'r2_bucket_name'").Scan(&bucketName)

	return &R2Config{
		Endpoint:        endpoint,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		BucketName:      bucketName,
	}, nil
}

// createS3Client 创建 S3 客户端
func createS3Client(r2Config *R2Config) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			r2Config.AccessKeyID,
			r2Config.SecretAccessKey,
			"",
		)),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(r2Config.Endpoint)
	})

	return client, nil
}

// GenerateUploadURL 生成上传预签名 URL
func (s *R2Service) GenerateUploadURL(key, contentType string, expiresIn time.Duration) (string, error) {
	log.Printf("[R2] 生成上传 URL: key=%s, contentType=%s", key, contentType)

	presignClient := s3.NewPresignClient(s.client)

	req, err := presignClient.PresignPutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})

	if err != nil {
		log.Printf("[R2] 生成上传 URL 失败: %v", err)
		return "", err
	}

	log.Printf("[R2] 上传 URL 生成成功")
	return req.URL, nil
}

// GenerateDownloadURL 生成下载预签名 URL
func (s *R2Service) GenerateDownloadURL(key, filename string, expiresIn time.Duration) (string, error) {
	log.Printf("[R2] 生成下载 URL: key=%s, filename=%s", key, filename)

	presignClient := s3.NewPresignClient(s.client)

	// 设置 Content-Disposition 以使用原始文件名下载
	contentDisposition := fmt.Sprintf("attachment; filename=\"%s\"", filename)

	req, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucketName),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String(contentDisposition),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})

	if err != nil {
		log.Printf("[R2] 生成下载 URL 失败: %v", err)
		return "", err
	}

	return req.URL, nil
}

// InitiateMultipartUpload 初始化分片上传
func (s *R2Service) InitiateMultipartUpload(key, contentType string) (string, error) {
	log.Printf("[R2] 初始化分片上传: key=%s", key)

	output, err := s.client.CreateMultipartUpload(context.TODO(), &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})

	if err != nil {
		log.Printf("[R2] 初始化分片上传失败: %v", err)
		return "", err
	}

	return *output.UploadId, nil
}

// GenerateMultipartUploadURL 生成分片上传预签名 URL
func (s *R2Service) GenerateMultipartUploadURL(key, uploadID string, partNumber int32) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	req, err := presignClient.PresignUploadPart(context.TODO(), &s3.UploadPartInput{
		Bucket:     aws.String(s.bucketName),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Hour
	})

	if err != nil {
		log.Printf("[R2] 生成分片上传 URL 失败: %v", err)
		return "", err
	}

	return req.URL, nil
}

// CompleteMultipartUpload 完成分片上传
func (s *R2Service) CompleteMultipartUpload(key, uploadID string, parts []types.CompletedPart) error {
	log.Printf("[R2] 完成分片上传: key=%s, parts=%d", key, len(parts))

	_, err := s.client.CompleteMultipartUpload(context.TODO(), &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.bucketName),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
	})

	if err != nil {
		log.Printf("[R2] 完成分片上传失败: %v", err)
	}

	return err
}

// ListParts 列出已上传的分片
func (s *R2Service) ListParts(key, uploadID string) ([]types.Part, error) {
	output, err := s.client.ListParts(context.TODO(), &s3.ListPartsInput{
		Bucket:   aws.String(s.bucketName),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})

	if err != nil {
		return nil, err
	}

	return output.Parts, nil
}

// DeleteObject 删除对象
func (s *R2Service) DeleteObject(key string) error {
	log.Printf("[R2] 删除对象: key=%s", key)

	_, err := s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		log.Printf("[R2] 删除对象失败: %v", err)
	}

	return err
}

// AbortMultipartUpload 终止分片上传
func (s *R2Service) AbortMultipartUpload(key, uploadID string) error {
	log.Printf("[R2] 终止分片上传: key=%s, uploadID=%s", key, uploadID)

	_, err := s.client.AbortMultipartUpload(context.TODO(), &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.bucketName),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})

	if err != nil {
		log.Printf("[R2] 终止分片上传失败: %v", err)
		return err
	}

	log.Println("[R2] 分片上传已终止")
	return nil
}

// TestConnection 测试 R2 连接
func (s *R2Service) TestConnection() error {
	log.Printf("[R2] 测试连接: bucket=%s", s.bucketName)

	// 尝试列出存储桶（只获取 1 个对象）
	_, err := s.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucketName),
		MaxKeys: aws.Int32(1),
	})

	if err != nil {
		log.Printf("[R2] 连接测试失败: %v", err)
		return fmt.Errorf("连接测试失败: %w", err)
	}

	log.Println("[R2] 连接测试成功")
	return nil
}

