package generate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const analysisCacheVersion = "gimbal-analysis-v1"
const maximumAnalysisEntry = 32 << 20

type analysisCache struct{ dir, producer string }
type analysisEnvelope struct {
	Version, Key, Checksum string
	Results                json.RawMessage
}

var producerOnce sync.Once
var producerDigest string

// Cache storage is optional: permission errors, corruption and unsupported
// source acquisition all retain ordinary generation. No error result is stored.
// The executable digest versions the extractor and all linked projection code,
// including development builds without release/VCS metadata.
func openAnalysisCache() *analysisCache {
	if os.Getenv("GIMBAL_GENERATE_CACHE") == "off" {
		return nil
	}
	dir := os.Getenv("GIMBAL_GENERATE_CACHE_DIR")
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil
		}
		dir = filepath.Join(base, "gimbal", "generate")
	}
	producerOnce.Do(func() {
		if path, err := os.Executable(); err == nil {
			producerDigest, _ = hashFile(path)
		}
	})
	if producerDigest == "" {
		return nil
	}
	return &analysisCache{dir: dir, producer: producerDigest}
}

func (c *analysisCache) key(in *analysisInputs, requests []analysisRequest) string {
	raw, _ := json.Marshal(struct {
		Version, Producer, Checker string
		Inputs                     inputManifest
		Requests                   []analysisRequest
	}{analysisCacheVersion, c.producer, runtime.Version(), in.manifest, requests})
	return hashBytes(raw)
}

// Keep only the latest successful source/producer version of each project query.
// Input changes replace this slot instead of accumulating one entry per edit.
func (c *analysisCache) path(in *analysisInputs, requests []analysisRequest) string {
	raw, _ := json.Marshal(struct {
		Dir      string
		Patterns []string
		Requests []analysisRequest
	}{in.manifest.Dir, in.manifest.Patterns, requests})
	return filepath.Join(c.dir, hashBytes(raw)+".json")
}

func (c *analysisCache) read(in *analysisInputs, requests []analysisRequest) ([]analysisResult, bool) {
	key := c.key(in, requests)
	path := c.path(in, requests)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maximumAnalysisEntry {
		return nil, false
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > maximumAnalysisEntry {
		return nil, false
	}
	var envelope analysisEnvelope
	if json.Unmarshal(raw, &envelope) != nil || envelope.Version != analysisCacheVersion || envelope.Key != key || hashBytes(envelope.Results) != envelope.Checksum {
		return nil, false
	}
	var result []analysisResult
	if json.Unmarshal(envelope.Results, &result) != nil || len(result) != len(requests) {
		return nil, false
	}
	return result, true
}

func (c *analysisCache) write(in *analysisInputs, requests []analysisRequest, result []analysisResult) {
	payload, err := json.Marshal(result)
	if err != nil {
		return
	}
	key := c.key(in, requests)
	raw, err := json.Marshal(analysisEnvelope{Version: analysisCacheVersion, Key: key, Checksum: hashBytes(payload), Results: payload})
	if err != nil || len(raw) > maximumAnalysisEntry {
		return
	}
	if os.MkdirAll(c.dir, 0700) != nil {
		return
	}
	file, err := os.CreateTemp(c.dir, ".analysis-*")
	if err != nil {
		return
	}
	defer func() { _ = os.Remove(file.Name()) }()
	_, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return
	}
	_ = os.Rename(file.Name(), c.path(in, requests))
}
