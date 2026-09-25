package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type fakeArtifactUploader struct {
	url string
	err error
}

func (u fakeArtifactUploader) upload(context.Context, string) (string, error) {
	return u.url, u.err
}

func uploaderConstructor(u artifactUploader) artifactUploaderConstructor {
	return func(context.Context, func(string) string) (artifactUploader, error) { return u, nil }
}

func TestUploadArtifactProviderSelectionAndOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      []string
		env       map[string]string
		wantR2    int
		wantS3    int
		wantCode  int
		wantOut   string
		wantError string
	}{
		{
			name:     "flag selects r2",
			args:     []string{"upload-artifact", "--provider", "r2", "proof.png"},
			wantR2:   1,
			wantCode: 0,
			wantOut:  "https://proof.example/r2\n",
		},
		{
			name:     "environment selects s3",
			args:     []string{"upload-artifact", "proof.png"},
			env:      map[string]string{"GIMBAL_UPLOAD_PROVIDER": "s3"},
			wantS3:   1,
			wantCode: 0,
			wantOut:  "https://proof.example/s3\n",
		},
		{
			name:     "flag takes precedence",
			args:     []string{"upload-artifact", "--provider", "r2", "proof.png"},
			env:      map[string]string{"GIMBAL_UPLOAD_PROVIDER": "s3"},
			wantR2:   1,
			wantCode: 0,
			wantOut:  "https://proof.example/r2\n",
		},
		{
			name:      "missing provider",
			args:      []string{"upload-artifact", "proof.png"},
			wantCode:  1,
			wantError: "gimbal: upload artifact: provider is required; use --provider r2|s3 or GIMBAL_UPLOAD_PROVIDER\n",
		},
		{
			name:      "unsupported provider",
			args:      []string{"upload-artifact", "--provider", "azure", "proof.png"},
			wantCode:  1,
			wantError: "gimbal: upload artifact: unsupported provider \"azure\"; use r2 or s3\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var r2Calls, s3Calls int
			constructors := artifactUploaderConstructors{
				r2: func(context.Context, func(string) string) (artifactUploader, error) {
					r2Calls++
					return fakeArtifactUploader{url: "https://proof.example/r2"}, nil
				},
				s3: func(context.Context, func(string) string) (artifactUploader, error) {
					s3Calls++
					return fakeArtifactUploader{url: "https://proof.example/s3"}, nil
				},
			}
			var stdout, stderr bytes.Buffer
			code := executeCLI(test.args, &stdout, &stderr, func(name string) string { return test.env[name] }, constructors)
			if code != test.wantCode || stdout.String() != test.wantOut || stderr.String() != test.wantError {
				t.Fatalf("executeCLI() = code %d, stdout %q, stderr %q; want %d, %q, %q", code, stdout.String(), stderr.String(), test.wantCode, test.wantOut, test.wantError)
			}
			if r2Calls != test.wantR2 || s3Calls != test.wantS3 {
				t.Fatalf("constructors called r2=%d s3=%d, want r2=%d s3=%d", r2Calls, s3Calls, test.wantR2, test.wantS3)
			}
		})
	}
}

func TestUploadArtifactFailureLeavesStdoutEmpty(t *testing.T) {
	t.Parallel()
	constructors := artifactUploaderConstructors{
		r2: uploaderConstructor(fakeArtifactUploader{err: errors.New("provider refused upload")}),
	}
	var stdout, stderr bytes.Buffer
	code := executeCLI([]string{"upload-artifact", "--provider", "r2", "proof.png"}, &stdout, &stderr, func(string) string { return "" }, constructors)
	if code != 1 || stdout.Len() != 0 || stderr.String() != "gimbal: upload artifact: provider refused upload\n" {
		t.Fatalf("executeCLI() = code %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestUploadArtifactConstructorFailureLeavesStdoutEmpty(t *testing.T) {
	t.Parallel()
	constructors := artifactUploaderConstructors{
		s3: func(context.Context, func(string) string) (artifactUploader, error) {
			return nil, errors.New("GIMBAL_S3_BUCKET is required")
		},
	}
	var stdout, stderr bytes.Buffer
	code := executeCLI([]string{"upload-artifact", "--provider", "s3", "proof.png"}, &stdout, &stderr, func(string) string { return "" }, constructors)
	if code != 1 || stdout.Len() != 0 || stderr.String() != "gimbal: upload artifact: configure s3: GIMBAL_S3_BUCKET is required\n" {
		t.Fatalf("executeCLI() = code %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestUploadArtifactConfigAndEnvironmentPrecedence(t *testing.T) {
	t.Parallel()
	fileConfig := artifactUploadConfig{
		Provider: "r2",
		R2: artifactUploadR2Config{
			AccountID:       "file-account",
			AccessKeyID:     "file-access",
			SecretAccessKey: "file-secret",
			Bucket:          "file-bucket",
			PublicBaseURL:   "https://file.example",
		},
	}
	var gotProvider, gotAccount, gotBucket string
	constructors := artifactUploaderConstructors{
		loadConfig: func() (artifactUploadConfig, error) { return fileConfig, nil },
		r2: func(_ context.Context, value func(string) string) (artifactUploader, error) {
			gotProvider = "r2"
			gotAccount = value("GIMBAL_R2_ACCOUNT_ID")
			gotBucket = value("GIMBAL_R2_BUCKET")
			return fakeArtifactUploader{url: "https://proof.example/r2"}, nil
		},
		s3: func(_ context.Context, value func(string) string) (artifactUploader, error) {
			gotProvider = "s3"
			gotBucket = value("GIMBAL_S3_BUCKET")
			return fakeArtifactUploader{url: "https://proof.example/s3"}, nil
		},
	}

	var stdout, stderr bytes.Buffer
	code := executeCLI([]string{"upload-artifact", "proof.png"}, &stdout, &stderr, func(name string) string {
		if name == "GIMBAL_R2_BUCKET" {
			return "environment-bucket"
		}
		return ""
	}, constructors)
	if code != 0 || gotProvider != "r2" || gotAccount != "file-account" || gotBucket != "environment-bucket" {
		t.Fatalf("config upload = code %d provider %q account %q bucket %q stderr %q", code, gotProvider, gotAccount, gotBucket, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	gotProvider = ""
	code = executeCLI([]string{"upload-artifact", "--provider", "s3", "proof.png"}, &stdout, &stderr, func(name string) string {
		if name == "GIMBAL_S3_BUCKET" {
			return "environment-s3-bucket"
		}
		return ""
	}, constructors)
	if code != 0 || gotProvider != "s3" || gotBucket != "environment-s3-bucket" {
		t.Fatalf("flag override = code %d provider %q bucket %q stderr %q", code, gotProvider, gotBucket, stderr.String())
	}
}

func TestUploadArtifactConfigFailureLeavesStdoutEmpty(t *testing.T) {
	t.Parallel()
	constructors := artifactUploaderConstructors{
		loadConfig: func() (artifactUploadConfig, error) { return artifactUploadConfig{}, errors.New("malformed config") },
	}
	var stdout, stderr bytes.Buffer
	code := executeCLI([]string{"upload-artifact", "--provider", "r2", "proof.png"}, &stdout, &stderr, func(string) string { return "" }, constructors)
	if code != 1 || stdout.Len() != 0 || stderr.String() != "gimbal: upload artifact: malformed config\n" {
		t.Fatalf("executeCLI() = code %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestReadArtifactUploadConfig(t *testing.T) {
	t.Parallel()
	missing, err := readArtifactUploadConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil || missing != (artifactUploadConfig{}) {
		t.Fatalf("missing config = %#v, %v", missing, err)
	}

	filename := filepath.Join(t.TempDir(), "config.json")
	contents := []byte(`{"uploadArtifact":{"provider":"r2","r2":{"accountId":"account","accessKeyId":"access","secretAccessKey":"secret","bucket":"bucket","publicBaseUrl":"https://public.example"},"s3":{"bucket":"s3-bucket","publicBaseUrl":"https://s3.example"}}}`)
	if err := os.WriteFile(filename, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := readArtifactUploadConfig(filename)
	if err != nil {
		t.Fatal(err)
	}
	if config.Provider != "r2" || config.R2.AccountID != "account" || config.R2.SecretAccessKey != "secret" || config.S3.Bucket != "s3-bucket" {
		t.Fatalf("config = %#v", config)
	}

	if err := os.WriteFile(filename, []byte(`{"uploadArtifact":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readArtifactUploadConfig(filename); err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("malformed config error = %v", err)
	}
}

func TestUploadArtifactHelpDescribesCallerContract(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := runWithUploaders([]string{"upload-artifact", "--help"}, &stdout, &stderr, func(string) string { return "" }, artifactUploaderConstructors{})
	if err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{
		"GIMBAL_UPLOAD_PROVIDER",
		"~/.gimbal/config.json",
		"Environment variables",
		"GIMBAL_R2_ACCOUNT_ID",
		"GIMBAL_S3_BUCKET",
		"normal region and credential",
		"publicly readable",
		"approximately 90 days",
		"never upload secrets",
		"does not provide TTL flags, listing",
		"stdout contains only that URL",
	} {
		if !strings.Contains(stdout.String(), phrase) {
			t.Errorf("help does not contain %q:\n%s", phrase, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("help stderr = %q", stderr.String())
	}
}

func TestR2EnvironmentValidation(t *testing.T) {
	t.Parallel()
	complete := map[string]string{
		"GIMBAL_R2_ACCOUNT_ID":        "account",
		"GIMBAL_R2_ACCESS_KEY_ID":     "access",
		"GIMBAL_R2_SECRET_ACCESS_KEY": "secret",
		"GIMBAL_R2_BUCKET":            "bucket",
		"GIMBAL_R2_PUBLIC_BASE_URL":   "https://artifacts.example/r2",
	}
	for missing := range complete {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()
			values := cloneStrings(complete)
			delete(values, missing)
			calls := map[string]int{}
			_, err := newR2UploaderWithClient(context.Background(), func(name string) string {
				calls[name]++
				return values[name]
			}, func(aws.Config, ...func(*s3.Options)) s3PutObjectClient {
				t.Fatal("client must not be constructed when configuration is invalid")
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), missing+" is required") {
				t.Fatalf("error = %v, want missing %s", err, missing)
			}
			for name := range complete {
				if calls[name] != 1 {
					t.Errorf("lookup %s called %d times, want once", name, calls[name])
				}
			}
		})
	}
}

func TestS3EnvironmentValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "bucket", env: map[string]string{"GIMBAL_S3_PUBLIC_BASE_URL": "https://example.com"}, want: "GIMBAL_S3_BUCKET is required"},
		{name: "public base", env: map[string]string{"GIMBAL_S3_BUCKET": "bucket"}, want: "GIMBAL_S3_PUBLIC_BASE_URL is required"},
		{name: "https", env: map[string]string{"GIMBAL_S3_BUCKET": "bucket", "GIMBAL_S3_PUBLIC_BASE_URL": "http://example.com"}, want: "must be an HTTPS base URL"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			loadCalls := 0
			_, err := newS3UploaderWithClient(context.Background(), func(name string) string { return test.env[name] }, func(context.Context, ...func(*config.LoadOptions) error) (aws.Config, error) {
				loadCalls++
				return aws.Config{}, nil
			}, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if loadCalls != 0 {
				t.Fatalf("AWS config loader called %d times for invalid provider config", loadCalls)
			}
		})
	}
}

type recordingS3Client struct {
	input  *s3.PutObjectInput
	body   []byte
	err    error
	stream bool
}

func (c *recordingS3Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	c.input = input
	_, c.stream = input.Body.(*os.File)
	if input.Body != nil {
		c.body, _ = io.ReadAll(input.Body)
	}
	return &s3.PutObjectOutput{}, c.err
}

func TestR2ConstructionRetainsIndependentConfiguration(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"GIMBAL_R2_ACCOUNT_ID":        "account-id",
		"GIMBAL_R2_ACCESS_KEY_ID":     "access-id",
		"GIMBAL_R2_SECRET_ACCESS_KEY": "secret-key",
		"GIMBAL_R2_BUCKET":            "r2-bucket",
		"GIMBAL_R2_PUBLIC_BASE_URL":   "https://cdn.example/r2 base",
	}
	calls := map[string]int{}
	client := &recordingS3Client{}
	var gotConfig aws.Config
	var gotOptions s3.Options
	uploader, err := newR2UploaderWithClient(context.Background(), func(name string) string {
		calls[name]++
		return values[name]
	}, func(cfg aws.Config, optionFns ...func(*s3.Options)) s3PutObjectClient {
		gotConfig = cfg
		for _, option := range optionFns {
			option(&gotOptions)
		}
		return client
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotConfig.Region != "auto" || aws.ToString(gotOptions.BaseEndpoint) != "https://account-id.r2.cloudflarestorage.com" || !gotOptions.UsePathStyle {
		t.Fatalf("R2 client config = region %q endpoint %q path-style %v", gotConfig.Region, aws.ToString(gotOptions.BaseEndpoint), gotOptions.UsePathStyle)
	}
	credential, err := gotConfig.Credentials.Retrieve(context.Background())
	if err != nil || credential.AccessKeyID != "access-id" || credential.SecretAccessKey != "secret-key" {
		t.Fatalf("R2 credentials = %#v, %v", credential, err)
	}

	for name := range values {
		values[name] = "changed"
	}
	uploader.now = func() time.Time { return time.Date(2026, 9, 21, 1, 2, 3, 0, time.UTC) }
	uploader.random = bytes.NewReader(make([]byte, 16))
	file := writeArtifact(t, "proof #1.png", []byte("r2 bytes"))
	publicURL, err := uploader.upload(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	if aws.ToString(client.input.Bucket) != "r2-bucket" || !client.stream || string(client.body) != "r2 bytes" {
		t.Fatalf("R2 upload = bucket %q stream %v body %q", aws.ToString(client.input.Bucket), client.stream, client.body)
	}
	wantKey := "artifacts/2026-09-21/00000000000000000000000000000000-proof #1.png"
	if aws.ToString(client.input.Key) != wantKey || aws.ToString(client.input.ContentType) != "image/png" || aws.ToString(client.input.ContentDisposition) != "inline" {
		t.Fatalf("R2 object = key %q content type %q disposition %q", aws.ToString(client.input.Key), aws.ToString(client.input.ContentType), aws.ToString(client.input.ContentDisposition))
	}
	if publicURL != "https://cdn.example/r2%20base/artifacts/2026-09-21/00000000000000000000000000000000-proof%20%231.png" {
		t.Fatalf("public URL = %q", publicURL)
	}
	for name := range calls {
		if calls[name] != 1 {
			t.Errorf("environment lookup %s called %d times after upload", name, calls[name])
		}
	}
}

func TestS3ConstructionPreservesAWSChainAndRetainsConfiguration(t *testing.T) {
	t.Parallel()
	values := map[string]string{
		"GIMBAL_S3_BUCKET":          "s3-bucket",
		"GIMBAL_S3_PUBLIC_BASE_URL": "https://cdn.example/s3",
	}
	calls := map[string]int{}
	client := &recordingS3Client{}
	chainCredentials := credentials.NewStaticCredentialsProvider("chain-access", "chain-secret", "chain-token")
	loadedConfig := aws.Config{Region: "profile-region", Credentials: chainCredentials}
	loadCalls := 0
	var clientConfig aws.Config
	uploader, err := newS3UploaderWithClient(context.Background(), func(name string) string {
		calls[name]++
		return values[name]
	}, func(_ context.Context, options ...func(*config.LoadOptions) error) (aws.Config, error) {
		loadCalls++
		if len(options) != 0 {
			t.Fatalf("AWS loader received %d overrides; want normal chain with none", len(options))
		}
		return loadedConfig, nil
	}, func(cfg aws.Config, options ...func(*s3.Options)) s3PutObjectClient {
		clientConfig = cfg
		if len(options) != 0 {
			t.Fatalf("S3 client received %d provider overrides", len(options))
		}
		return client
	})
	if err != nil {
		t.Fatal(err)
	}
	credential, credentialErr := clientConfig.Credentials.Retrieve(context.Background())
	if loadCalls != 1 || clientConfig.Region != "profile-region" || credentialErr != nil || credential.AccessKeyID != "chain-access" || credential.SecretAccessKey != "chain-secret" || credential.SessionToken != "chain-token" {
		t.Fatalf("S3 construction = load calls %d, region %q, credentials %#v, error %v", loadCalls, clientConfig.Region, credential, credentialErr)
	}

	values["GIMBAL_S3_BUCKET"] = "changed"
	values["GIMBAL_S3_PUBLIC_BASE_URL"] = "https://changed.example"
	uploader.now = func() time.Time { return time.Date(2026, 9, 21, 0, 0, 0, 0, time.FixedZone("west", -6*60*60)) }
	uploader.random = bytes.NewReader(bytes.Repeat([]byte{0xab}, 16))
	file := writeArtifact(t, "archive.unknown", []byte("s3 bytes"))
	publicURL, err := uploader.upload(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	wantKey := "artifacts/2026-09-21/abababababababababababababababab-archive.unknown"
	if aws.ToString(client.input.Bucket) != "s3-bucket" || aws.ToString(client.input.Key) != wantKey || aws.ToString(client.input.ContentType) != "application/octet-stream" || aws.ToString(client.input.ContentDisposition) != "inline" {
		t.Fatalf("S3 object = bucket %q key %q content type %q disposition %q", aws.ToString(client.input.Bucket), aws.ToString(client.input.Key), aws.ToString(client.input.ContentType), aws.ToString(client.input.ContentDisposition))
	}
	if !client.stream || string(client.body) != "s3 bytes" || publicURL != "https://cdn.example/s3/"+wantKey {
		t.Fatalf("S3 upload = stream %v body %q URL %q", client.stream, client.body, publicURL)
	}
	for name := range calls {
		if calls[name] != 1 {
			t.Errorf("environment lookup %s called %d times after upload", name, calls[name])
		}
	}
}

func TestArtifactContentType(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		want string
	}{
		{name: "proof.md", want: "text/markdown; charset=utf-8"},
		{name: "proof.MARKDOWN", want: "text/markdown; charset=utf-8"},
		{name: "proof.png", want: "image/png"},
		{name: "proof.unknown", want: "application/octet-stream"},
	} {
		if got := artifactContentType(test.name); got != test.want {
			t.Errorf("artifactContentType(%q) = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestArtifactUploadRejectsUnreadableAndNonRegularInputs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.png")
	for _, test := range []struct {
		name string
		path string
		want string
	}{
		{name: "unreadable", path: missing, want: "open"},
		{name: "non regular", path: dir, want: "is not a regular file"},
	} {
		for _, provider := range []string{"r2", "s3"} {
			t.Run(provider+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				client := &recordingS3Client{}
				uploader := testUploader(provider, client)
				url, err := uploader.upload(context.Background(), test.path)
				if err == nil || !strings.Contains(err.Error(), test.want) || url != "" {
					t.Fatalf("upload() = %q, %v; want empty URL and %q error", url, err, test.want)
				}
				if client.input != nil {
					t.Fatal("provider was called for invalid local input")
				}
			})
		}
	}
}

func TestArtifactUploadProviderFailuresReturnNoURL(t *testing.T) {
	t.Parallel()
	file := writeArtifact(t, "proof.txt", []byte("proof"))
	for _, provider := range []string{"r2", "s3"} {
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			client := &recordingS3Client{err: errors.New("remote failure")}
			uploader := testUploader(provider, client)
			url, err := uploader.upload(context.Background(), file)
			if err == nil || !strings.Contains(err.Error(), "remote failure") || url != "" {
				t.Fatalf("upload() = %q, %v; want empty URL and provider error", url, err)
			}
		})
	}
}

func testUploader(provider string, client s3PutObjectClient) artifactUploader {
	base, _ := parsePublicBaseURL("https://example.com/public", "test")
	if provider == "r2" {
		return &r2Uploader{client: client, bucket: "bucket", publicBase: base, now: time.Now, random: bytes.NewReader(make([]byte, 16))}
	}
	return &s3Uploader{client: client, bucket: "bucket", publicBase: base, now: time.Now, random: bytes.NewReader(make([]byte, 16))}
}

func writeArtifact(t *testing.T, name string, contents []byte) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(filename, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	return filename
}

func cloneStrings(source map[string]string) map[string]string {
	clone := make(map[string]string, len(source))
	maps.Copy(clone, source)
	return clone
}
