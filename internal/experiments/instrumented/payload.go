package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/compiledscope"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/proto"
)

// Storage belongs to the transport, not authored workflow code. Immutable
// objects must remain available to clients and replay workers for this history.
// Serialization context chooses the run directory; the reference is self
// describing because history readers do not necessarily have that context.
type fileCodec struct{ Root, Run string }
type payloadRef struct {
	Run      string
	Snapshot compiledscope.Snapshot
}

func dataConverter(root string) converter.DataConverter {
	return converter.NewCodecDataConverter(converter.GetDefaultDataConverter(), fileCodec{Root: root})
}
func (c fileCodec) WithSerializationContext(sc converter.SerializationContext) converter.PayloadCodec {
	switch v := sc.(type) {
	case converter.WorkflowSerializationContext:
		c.Run = environmentID(v.WorkflowID)
	case converter.ActivitySerializationContext:
		c.Run = environmentID(v.WorkflowID)
	}
	return c
}
func (c fileCodec) Encode(in []*commonpb.Payload) ([]*commonpb.Payload, error) {
	out := make([]*commonpb.Payload, len(in))
	for i, p := range in {
		out[i] = p
		if len(p.Data) < 64<<10 {
			continue
		}
		if c.Run == "" {
			return nil, fmt.Errorf("large payload requires run serialization context")
		}
		b, err := proto.MarshalOptions{Deterministic: true}.Marshal(p)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(b)
		if err != nil {
			return nil, err
		}
		s := compiledscope.Store{Root: filepath.Join(c.Root, c.Run, "context")}
		ref, err := s.Extend(context.Background(), "", compiledscope.Entry{Key: "payload", Value: raw})
		if err != nil {
			return nil, err
		}
		b, err = json.Marshal(payloadRef{c.Run, ref})
		if err != nil {
			return nil, err
		}
		out[i] = &commonpb.Payload{Metadata: map[string][]byte{converter.MetadataEncoding: []byte("gimbal/file-v1")}, Data: b}
	}
	return out, nil
}
func (c fileCodec) Decode(in []*commonpb.Payload) ([]*commonpb.Payload, error) {
	out := make([]*commonpb.Payload, len(in))
	for i, p := range in {
		out[i] = p
		if string(p.Metadata[converter.MetadataEncoding]) != "gimbal/file-v1" {
			continue
		}
		var ref payloadRef
		if err := json.Unmarshal(p.Data, &ref); err != nil {
			return nil, err
		}
		if filepath.Base(ref.Run) != ref.Run || len(ref.Run) != len("gimbal-specimen-")+16 {
			return nil, fmt.Errorf("invalid payload run")
		}
		s := compiledscope.Store{Root: filepath.Join(c.Root, ref.Run, "context")}
		entries, err := s.Load(context.Background(), ref.Snapshot)
		if err != nil {
			return nil, err
		}
		if len(entries) != 1 || entries[0].Key != "payload" {
			return nil, fmt.Errorf("invalid payload manifest")
		}
		raw, err := s.Value(context.Background(), entries[0])
		if err != nil {
			return nil, err
		}
		var b []byte
		if err = json.Unmarshal(raw, &b); err != nil {
			return nil, err
		}
		out[i] = &commonpb.Payload{}
		if err = proto.Unmarshal(b, out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}
