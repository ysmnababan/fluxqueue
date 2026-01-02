// Package storage provides functions to interact with object storage services using MinIO.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client     *minio.Client
	BucketName string
}

func NewStorage(endpoint, accessKeyID, secretAccessKey string, useSSL bool) *Storage {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}
	return &Storage{
		client: minioClient,
		// BucketName: bucketName,
	}
}

func (s *Storage) CreateBucketWithCheck(ctx context.Context, bucketName string) error {
	location := "us-east-1"
	err := s.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		exists, errBucketExists := s.client.BucketExists(context.Background(), bucketName)
		if errBucketExists == nil && exists {
			err = errors.New("Bucket already exists")
		}
		return err
	}
	fmt.Println("Bucket created successfully!")
	return nil
}

// PutObject
//
// ObjectName can contains path too: i.e. /path1/path2/readme.txt
func (s *Storage) PutObject(ctx context.Context, filepath, bucketname, objectName, contentType string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}

	defer func() {
		err = file.Close()
		if err != nil {
			fmt.Println("err: ", err)
		}
	}()

	fileStat, err := file.Stat()
	if err != nil {
		return err
	}
	uploadInfo, err := s.client.PutObject(ctx, bucketname, objectName, file, fileStat.Size(),
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return err
	}
	fmt.Println("Info upload: ", uploadInfo)
	return nil
}

func (s *Storage) GetObject(ctx context.Context, bucketName, objectName string) error {
	obj, err := s.client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return err
	}

	defer func() {
		err = obj.Close()
		if err != nil {
			fmt.Println("err: ", err)
		}
	}()

	localFile, err := os.Create("/tmp/download.xlsx")
	if err != nil {
		return err
	}
	defer func() {
		err = localFile.Close()
		if err != nil {
			fmt.Println("err: ", err)
		}
	}()

	if _, err := io.Copy(localFile, obj); err != nil {
		return err
	}
	fmt.Println("download success")
	return nil
}

// ListObjects
//
// Prefix can be filepath or filename
func (s *Storage) ListObjects(ctx context.Context, bucketName string, prefix string) error {
	objectCh := s.client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	for object := range objectCh {
		if object.Err != nil {
			fmt.Println("error while streaming the response from the object: ", object.Err)
			return object.Err
		}
		fmt.Println(object)
	}
	return nil
}

// FileExists
//
// ObjectName must be exact path and filename
func (s *Storage) FileExists(ctx context.Context, bucketName, objectName string) (bool, error) {
	_, err := s.client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil // File not found
		}
		return false, err // Other error
	}
	return true, nil
}

func (s *Storage) GetPresignedURL(ctx context.Context, bucket, object string) (string, error) {
	reqParams := make(url.Values)
	u, err := s.client.PresignedGetObject(ctx, bucket, object, 3*time.Minute, reqParams)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
