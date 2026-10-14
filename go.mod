module github.com/tylergannon/gimbal

go 1.27.2

// web/ is a Go package (it embeds the build), so without this every ./... walk
// descends into web/node_modules looking for Go packages.
ignore ./web/node_modules

// Downloaded reference source and the pinned oracle are separate workspaces.
ignore ./ephemeral/inspiration

ignore ./third_party/opencode/oracle/upstream

require (
	github.com/aws/aws-sdk-go-v2 v1.47.1
	github.com/aws/aws-sdk-go-v2/config v1.33.7
	github.com/aws/aws-sdk-go-v2/credentials v1.20.7
	github.com/aws/aws-sdk-go-v2/service/s3 v1.114.1
	github.com/coder/websocket v1.8.15
	github.com/jackc/pgx/v5 v5.11.0
	github.com/kazz187/jev-sdk-go v0.3.0
	github.com/oapi-codegen/runtime v1.7.0
	github.com/oklog/ulid/v2 v2.1.2
	github.com/riverqueue/river v0.49.0
	github.com/riverqueue/river/riverdriver/riverpgxv5 v0.49.0
	github.com/riverqueue/river/rivertype v0.49.0
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/tiktoken-go/tokenizer v0.8.1
	github.com/tylergannon/claude-agent-sdk-go v1.2.1
	github.com/tylergannon/devalue/v5 v5.0.0
	github.com/tylergannon/polytype v1.9.0
	github.com/tylergannon/skgo v0.33.0
	github.com/yuin/goldmark v1.7.13
	golang.org/x/net v0.61.0
	golang.org/x/sync v0.24.0
	golang.org/x/sys v0.49.0
	golang.org/x/text v0.43.0
	golang.org/x/tools v0.52.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.2 // indirect
	github.com/aws/smithy-go v1.28.4 // indirect
	github.com/dave/dst v0.28.0 // indirect
	github.com/dlclark/regexp2/v2 v2.8.3 // indirect
	github.com/dop251/goja v0.0.0-20261008222547-e9f698900731 // indirect
	github.com/dop251/goja_nodejs v0.0.0-20260918173711-b481721df8a2 // indirect
	github.com/dprotaso/go-yit v0.0.0-20220510233725-9ba8df137936 // indirect
	github.com/getkin/kin-openapi v0.149.0 // indirect
	github.com/go-openapi/jsonpointer v1.0.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible // indirect
	github.com/google/pprof v0.0.0-20261006160405-d99a6174ef52 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/oapi-codegen/oapi-codegen/v2 v2.8.0 // indirect
	github.com/oasdiff/yaml v0.1.1 // indirect
	github.com/oasdiff/yaml3 v0.0.14 // indirect
	github.com/riverqueue/river/riverdriver v0.49.0 // indirect
	github.com/riverqueue/river/rivershared v0.49.0 // indirect
	github.com/speakeasy-api/jsonpath v0.6.3 // indirect
	github.com/speakeasy-api/openapi v1.25.5 // indirect
	github.com/tidwall/gjson v1.19.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/tylergannon/structtag v0.1.0 // indirect
	github.com/vmware-labs/yaml-jsonpath v0.3.2 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/telemetry v0.0.0-20261009151841-97e8ea77aa2a // indirect
	golang.org/x/term v0.47.0 // indirect
)

tool (
	github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
	github.com/tylergannon/polytype/polytype
	// skgo generates the bindings, and projects the Go types that cross to
	// TypeScript through polytype's library. It runs through `go tool`, so it is
	// built from the module cache and does not have to be a writable checkout.
	github.com/tylergannon/skgo/cmd/skgo
	golang.org/x/tools/cmd/goimports
	golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize
)
