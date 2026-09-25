package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
)

type artifactUploader interface {
	upload(context.Context, string) (string, error)
}

type artifactUploaderConstructor func(context.Context, func(string) string) (artifactUploader, error)

type artifactUploaderConstructors struct {
	r2         artifactUploaderConstructor
	s3         artifactUploaderConstructor
	loadConfig func() (artifactUploadConfig, error)
}

func defaultArtifactUploaders() artifactUploaderConstructors {
	return artifactUploaderConstructors{r2: newR2Uploader, s3: newS3Uploader, loadConfig: loadArtifactUploadConfig}
}

type artifactUploadConfig struct {
	Provider string                 `json:"provider"`
	R2       artifactUploadR2Config `json:"r2"`
	S3       artifactUploadS3Config `json:"s3"`
}

type artifactUploadR2Config struct {
	AccountID       string `json:"accountId"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	Bucket          string `json:"bucket"`
	PublicBaseURL   string `json:"publicBaseUrl"`
}

type artifactUploadS3Config struct {
	Bucket        string `json:"bucket"`
	PublicBaseURL string `json:"publicBaseUrl"`
}

func newUploadArtifactCommand(stdout io.Writer, getenv func(string) string, constructors artifactUploaderConstructors) *cobra.Command {
	var providerFlag string
	cmd := &cobra.Command{
		Use:   "upload-artifact --provider r2|s3 PATH_TO_FILE",
		Short: "Upload one public, temporary proof artifact",
		Long: `Upload one regular local file to a configured R2 or AWS S3 bucket and print
its public HTTPS URL. On success, stdout contains only that URL.

Select r2 or s3 with --provider. If the flag is absent,
GIMBAL_UPLOAD_PROVIDER supplies the provider, followed by the provider in
~/.gimbal/config.json. Credentials never select it. Environment variables
override corresponding file values, and --provider overrides both.

The optional config file has this shape:
  {
    "uploadArtifact": {
      "provider": "r2",
      "r2": {
        "accountId": "...",
        "accessKeyId": "...",
        "secretAccessKey": "...",
        "bucket": "...",
        "publicBaseUrl": "https://..."
      }
    }
  }
Because it can contain credentials, keep the file private to your user.

R2 requires GIMBAL_R2_ACCOUNT_ID, GIMBAL_R2_ACCESS_KEY_ID,
GIMBAL_R2_SECRET_ACCESS_KEY, GIMBAL_R2_BUCKET, and
GIMBAL_R2_PUBLIC_BASE_URL. S3 requires GIMBAL_S3_BUCKET and
GIMBAL_S3_PUBLIC_BASE_URL and uses the AWS SDK's normal region and credential
chain, including AWS_REGION, environment credentials, and configured profiles.

The selected bucket or artifact prefix must already be publicly readable and
have a lifecycle rule that removes artifacts after approximately 90 days.
Gimbal does not inspect or change access policy or retention. Anyone with the
returned URL can read the file, so never upload secrets or sensitive material.

This command uploads one file. It does not provide TTL flags, listing,
deletion, or multi-file upload.

Examples:
  gimbal upload-artifact --provider r2 screenshot.png
  GIMBAL_UPLOAD_PROVIDER=s3 gimbal upload-artifact report.txt`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fileConfig := artifactUploadConfig{}
			if constructors.loadConfig != nil {
				var err error
				fileConfig, err = constructors.loadConfig()
				if err != nil {
					return fmt.Errorf("upload artifact: %w", err)
				}
			}
			configuredValue := fileConfig.value(getenv)
			provider := providerFlag
			if provider == "" {
				provider = configuredValue("GIMBAL_UPLOAD_PROVIDER")
			}

			var constructor artifactUploaderConstructor
			switch provider {
			case "r2":
				constructor = constructors.r2
			case "s3":
				constructor = constructors.s3
			case "":
				return errors.New("upload artifact: provider is required; use --provider r2|s3 or GIMBAL_UPLOAD_PROVIDER")
			default:
				return fmt.Errorf("upload artifact: unsupported provider %q; use r2 or s3", provider)
			}

			uploader, err := constructor(cmd.Context(), configuredValue)
			if err != nil {
				return fmt.Errorf("upload artifact: configure %s: %w", provider, err)
			}
			publicURL, err := uploader.upload(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("upload artifact: %w", err)
			}
			_, err = fmt.Fprintln(stdout, publicURL)
			return err
		},
	}
	cmd.Flags().StringVar(&providerFlag, "provider", "", "storage provider: r2 or s3 (default GIMBAL_UPLOAD_PROVIDER)")
	return cmd
}

func loadArtifactUploadConfig() (artifactUploadConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return artifactUploadConfig{}, fmt.Errorf("find home directory for artifact config: %w", err)
	}
	return readArtifactUploadConfig(filepath.Join(home, ".gimbal", "config.json"))
}

func readArtifactUploadConfig(filename string) (artifactUploadConfig, error) {
	data, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return artifactUploadConfig{}, nil
	}
	if err != nil {
		return artifactUploadConfig{}, fmt.Errorf("read %q: %w", filename, err)
	}
	var config struct {
		UploadArtifact artifactUploadConfig `json:"uploadArtifact"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return artifactUploadConfig{}, fmt.Errorf("parse %q: %w", filename, err)
	}
	return config.UploadArtifact, nil
}

func (c artifactUploadConfig) value(getenv func(string) string) func(string) string {
	return func(name string) string {
		if value := getenv(name); value != "" {
			return value
		}
		switch name {
		case "GIMBAL_UPLOAD_PROVIDER":
			return c.Provider
		case "GIMBAL_R2_ACCOUNT_ID":
			return c.R2.AccountID
		case "GIMBAL_R2_ACCESS_KEY_ID":
			return c.R2.AccessKeyID
		case "GIMBAL_R2_SECRET_ACCESS_KEY":
			return c.R2.SecretAccessKey
		case "GIMBAL_R2_BUCKET":
			return c.R2.Bucket
		case "GIMBAL_R2_PUBLIC_BASE_URL":
			return c.R2.PublicBaseURL
		case "GIMBAL_S3_BUCKET":
			return c.S3.Bucket
		case "GIMBAL_S3_PUBLIC_BASE_URL":
			return c.S3.PublicBaseURL
		default:
			return ""
		}
	}
}

type s3PutObjectClient interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type r2Uploader struct {
	client     s3PutObjectClient
	bucket     string
	publicBase *url.URL
	now        func() time.Time
	random     io.Reader
}

func newR2Uploader(ctx context.Context, getenv func(string) string) (artifactUploader, error) {
	return newR2UploaderWithClient(ctx, getenv, func(cfg aws.Config, options ...func(*s3.Options)) s3PutObjectClient {
		return s3.NewFromConfig(cfg, options...)
	})
}

func newR2UploaderWithClient(_ context.Context, getenv func(string) string, newClient func(aws.Config, ...func(*s3.Options)) s3PutObjectClient) (*r2Uploader, error) {
	accountID := getenv("GIMBAL_R2_ACCOUNT_ID")
	accessKeyID := getenv("GIMBAL_R2_ACCESS_KEY_ID")
	secretAccessKey := getenv("GIMBAL_R2_SECRET_ACCESS_KEY")
	bucket := getenv("GIMBAL_R2_BUCKET")
	publicBaseValue := getenv("GIMBAL_R2_PUBLIC_BASE_URL")
	for _, variable := range []struct {
		name  string
		value string
	}{
		{name: "GIMBAL_R2_ACCOUNT_ID", value: accountID},
		{name: "GIMBAL_R2_ACCESS_KEY_ID", value: accessKeyID},
		{name: "GIMBAL_R2_SECRET_ACCESS_KEY", value: secretAccessKey},
		{name: "GIMBAL_R2_BUCKET", value: bucket},
		{name: "GIMBAL_R2_PUBLIC_BASE_URL", value: publicBaseValue},
	} {
		if variable.value == "" {
			return nil, fmt.Errorf("%s is required", variable.name)
		}
	}
	publicBase, err := parsePublicBaseURL(publicBaseValue, "GIMBAL_R2_PUBLIC_BASE_URL")
	if err != nil {
		return nil, err
	}

	cfg := aws.Config{
		Region:      "auto",
		Credentials: credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
	}
	endpoint := "https://" + accountID + ".r2.cloudflarestorage.com"
	client := newClient(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	return &r2Uploader{client: client, bucket: bucket, publicBase: publicBase, now: time.Now, random: rand.Reader}, nil
}

func (u *r2Uploader) upload(ctx context.Context, filename string) (string, error) {
	file, info, err := openArtifact(filename)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	key, err := artifactKey(info.Name(), u.now(), u.random)
	if err != nil {
		return "", err
	}
	contentType := artifactContentType(info.Name())
	if _, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:             aws.String(u.bucket),
		Key:                aws.String(key),
		Body:               file,
		ContentDisposition: aws.String("inline"),
		ContentType:        aws.String(contentType),
	}); err != nil {
		return "", fmt.Errorf("upload %q to R2: %w", filename, err)
	}
	return publicArtifactURL(u.publicBase, key), nil
}

type awsConfigLoader func(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error)

type s3Uploader struct {
	client     s3PutObjectClient
	bucket     string
	publicBase *url.URL
	now        func() time.Time
	random     io.Reader
}

func newS3Uploader(ctx context.Context, getenv func(string) string) (artifactUploader, error) {
	return newS3UploaderWithClient(ctx, getenv, config.LoadDefaultConfig, func(cfg aws.Config, options ...func(*s3.Options)) s3PutObjectClient {
		return s3.NewFromConfig(cfg, options...)
	})
}

func newS3UploaderWithClient(ctx context.Context, getenv func(string) string, loadConfig awsConfigLoader, newClient func(aws.Config, ...func(*s3.Options)) s3PutObjectClient) (*s3Uploader, error) {
	bucket := getenv("GIMBAL_S3_BUCKET")
	publicBaseValue := getenv("GIMBAL_S3_PUBLIC_BASE_URL")
	if bucket == "" {
		return nil, errors.New("GIMBAL_S3_BUCKET is required")
	}
	if publicBaseValue == "" {
		return nil, errors.New("GIMBAL_S3_PUBLIC_BASE_URL is required")
	}
	publicBase, err := parsePublicBaseURL(publicBaseValue, "GIMBAL_S3_PUBLIC_BASE_URL")
	if err != nil {
		return nil, err
	}
	cfg, err := loadConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load the AWS region and credential chain: %w", err)
	}
	return &s3Uploader{
		client:     newClient(cfg),
		bucket:     bucket,
		publicBase: publicBase,
		now:        time.Now,
		random:     rand.Reader,
	}, nil
}

func (u *s3Uploader) upload(ctx context.Context, filename string) (string, error) {
	file, info, err := openArtifact(filename)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	key, err := artifactKey(info.Name(), u.now(), u.random)
	if err != nil {
		return "", err
	}
	contentType := artifactContentType(info.Name())
	if _, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:             aws.String(u.bucket),
		Key:                aws.String(key),
		Body:               file,
		ContentDisposition: aws.String("inline"),
		ContentType:        aws.String(contentType),
	}); err != nil {
		return "", fmt.Errorf("upload %q to S3: %w", filename, err)
	}
	return publicArtifactURL(u.publicBase, key), nil
}

func openArtifact(filename string) (*os.File, os.FileInfo, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("open %q: %w", filename, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("inspect %q: %w", filename, err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, fmt.Errorf("%q is not a regular file", filename)
	}
	return file, info, nil
}

func artifactKey(basename string, now time.Time, random io.Reader) (string, error) {
	randomID := make([]byte, 16)
	if _, err := io.ReadFull(random, randomID); err != nil {
		return "", fmt.Errorf("create random artifact identifier: %w", err)
	}
	return path.Join("artifacts", now.UTC().Format(time.DateOnly), hex.EncodeToString(randomID)+"-"+safeArtifactBasename(basename)), nil
}

func safeArtifactBasename(basename string) string {
	basename = filepath.Base(basename)
	var result strings.Builder
	for _, r := range basename {
		if r == '\\' || unicode.IsControl(r) {
			result.WriteRune('_')
			continue
		}
		result.WriteRune(r)
	}
	if result.Len() == 0 || result.String() == "." {
		return "artifact"
	}
	return result.String()
}

func artifactContentType(basename string) string {
	extension := strings.ToLower(filepath.Ext(basename))
	if extension == ".md" || extension == ".markdown" {
		return "text/markdown; charset=utf-8"
	}
	if contentType := mime.TypeByExtension(extension); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func parsePublicBaseURL(value, variable string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("%s must be an HTTPS base URL without credentials, query, or fragment", variable)
	}
	return parsed, nil
}

func publicArtifactURL(base *url.URL, key string) string {
	result := *base
	result.Path = path.Join(result.Path, key)
	result.RawPath = ""
	return result.String()
}
