// Package projectfiles stores durable project file metadata on the server and
// original file bodies in the configured R2 bucket.
package projectfiles

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/tylergannon/gimbal/internal/transcript"
)

type Document struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
}

// Uses the same ~/.gimbal/config.json settings and environment precedence as upload-artifact.
func client() (*s3.Client, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", err
	}
	var config struct {
		UploadArtifact struct {
			R2 struct {
				AccountID       string `json:"accountId"`
				AccessKeyID     string `json:"accessKeyId"`
				SecretAccessKey string `json:"secretAccessKey"`
				Bucket          string `json:"bucket"`
			}
		} `json:"uploadArtifact"`
	}
	data, err := os.ReadFile(filepath.Join(home, ".gimbal", "config.json"))
	if err != nil && !os.IsNotExist(err) {
		return nil, "", err
	}
	if err == nil {
		if err = json.Unmarshal(data, &config); err != nil {
			return nil, "", err
		}
	}
	override := func(name, fallback string) string {
		if value := os.Getenv(name); value != "" {
			return value
		}
		return fallback
	}
	r := config.UploadArtifact.R2
	bucket := override("GIMBAL_R2_BUCKET", r.Bucket)
	account := override("GIMBAL_R2_ACCOUNT_ID", r.AccountID)
	key := override("GIMBAL_R2_ACCESS_KEY_ID", r.AccessKeyID)
	secret := override("GIMBAL_R2_SECRET_ACCESS_KEY", r.SecretAccessKey)
	if bucket == "" || account == "" || key == "" || secret == "" {
		return nil, "", errors.New("configure R2 in ~/.gimbal/config.json under uploadArtifact.r2")
	}
	c := s3.New(s3.Options{Region: "auto", Credentials: credentials.NewStaticCredentialsProvider(key, secret, ""), BaseEndpoint: aws.String("https://" + account + ".r2.cloudflarestorage.com"), UsePathStyle: true})
	return c, bucket, nil
}
func ViewURL(ctx context.Context, f Document) (string, error) {
	c, _, err := client()
	if err != nil {
		return "", err
	}
	result, err := s3.NewPresignClient(c).PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(f.Bucket), Key: aws.String(f.Key)}, func(o *s3.PresignOptions) { o.Expires = 15 * time.Minute })
	if err != nil {
		return "", err
	}
	return result.URL, nil
}
func Upload(ctx context.Context, dir, project, name string, data []byte) (Document, error) {
	if name == "" {
		return Document{}, errors.New("choose a file")
	}
	c, bucket, err := client()
	if err != nil {
		return Document{}, err
	}
	idBytes := make([]byte, 16)
	if _, err = rand.Read(idBytes); err != nil {
		return Document{}, err
	}
	id := hex.EncodeToString(idBytes)
	name = filepath.Base(name)
	media := mime.TypeByExtension(filepath.Ext(name))
	if media == "" {
		media = http.DetectContentType(data)
	}
	file := Document{ID: id, Name: name, MediaType: media, Size: int64(len(data)), Bucket: bucket, Key: "projects/" + project + "/" + id + "/" + name}
	if _, err = c.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(file.Key), Body: bytes.NewReader(data), ContentType: aws.String(media)}); err != nil {
		return Document{}, err
	}
	if err = os.MkdirAll(filepath.Join(dir, "files"), 0755); err != nil {
		return Document{}, err
	}
	return file, transcript.WriteJSON(filepath.Join(dir, "files", id+".json"), file)
}
func List(dir string) ([]Document, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "files"))
	if os.IsNotExist(err) {
		return []Document{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := []Document{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, "files", e.Name()))
		if err != nil {
			return nil, err
		}
		var f Document
		if err = json.Unmarshal(data, &f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	slices.SortFunc(files, func(a, b Document) int { return strings.Compare(a.Name, b.Name) })
	return files, nil
}
func Get(dir, id string) (Document, error) {
	if len(id) != 32 {
		return Document{}, errors.New("invalid file ID")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return Document{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "files", id+".json"))
	if err != nil {
		return Document{}, err
	}
	var f Document
	err = json.Unmarshal(data, &f)
	return f, err
}
func Body(ctx context.Context, f Document, byteRange string) (io.ReadCloser, int64, string, error) {
	c, _, err := client()
	if err != nil {
		return nil, 0, "", err
	}
	input := &s3.GetObjectInput{Bucket: aws.String(f.Bucket), Key: aws.String(f.Key)}
	if byteRange != "" {
		input.Range = aws.String(byteRange)
	}
	obj, err := c.GetObject(ctx, input)
	if err != nil {
		return nil, 0, "", err
	}
	return obj.Body, aws.ToInt64(obj.ContentLength), aws.ToString(obj.ContentRange), nil
}
