package asrpb

import (
	"context"
	"fmt"
	"math"

	"github.com/SpherePrime/CLI/vendordeps/grpc"
	"github.com/SpherePrime/CLI/vendordeps/grpc/encoding"
	"github.com/SpherePrime/CLI/vendordeps/grpc/mem"
	"github.com/SpherePrime/CLI/vendordeps/protobuf/encoding/protowire"
)

const (
	TokenHeader         = "x-prime-asr-token"
	methodStatus        = "/prime.asr.v1.AsrService/Status"
	methodStream        = "/prime.asr.v1.AsrService/StreamRecognize"
	methodTranscribe    = "/prime.asr.v1.AsrService/Transcribe"
	methodShutdown      = "/prime.asr.v1.AsrService/Shutdown"
)

type wireMessage interface {
	marshalWire() []byte
	unmarshalWire([]byte) error
}

type protoCodec struct{}

func Codec() encoding.CodecV2 {
	return protoCodec{}
}

func (protoCodec) Name() string {
	return "proto"
}

func (protoCodec) Marshal(v any) (mem.BufferSlice, error) {
	msg, ok := v.(wireMessage)
	if !ok {
		return nil, fmt.Errorf("asrpb: unsupported message %T", v)
	}
	return mem.BufferSlice{mem.SliceBuffer(msg.marshalWire())}, nil
}

func (protoCodec) Unmarshal(data mem.BufferSlice, v any) error {
	msg, ok := v.(wireMessage)
	if !ok {
		return fmt.Errorf("asrpb: unsupported message %T", v)
	}
	buf := data.MaterializeToBuffer(mem.DefaultBufferPool())
	defer buf.Free()
	return msg.unmarshalWire(buf.ReadOnlyData())
}

type StatusRequest struct{}

func (*StatusRequest) marshalWire() []byte      { return nil }
func (*StatusRequest) unmarshalWire([]byte) error { return nil }

type StatusResponse struct {
	Ready      bool
	Languages  []string
	ChunkMs    uint32
	SampleRate uint32
}

func (m *StatusResponse) marshalWire() []byte {
	var b []byte
	if m.Ready {
		b = protowire.AppendTag(b, 1, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	for _, lang := range m.Languages {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, lang)
	}
	if m.ChunkMs != 0 {
		b = protowire.AppendTag(b, 3, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(m.ChunkMs))
	}
	if m.SampleRate != 0 {
		b = protowire.AppendTag(b, 4, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(m.SampleRate))
	}
	return b
}

func (m *StatusResponse) unmarshalWire(b []byte) error {
	*m = StatusResponse{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, mlen := protowire.ConsumeVarint(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.Ready = v != 0
			b = b[mlen:]
		case 2:
			v, mlen := protowire.ConsumeString(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.Languages = append(m.Languages, v)
			b = b[mlen:]
		case 3:
			v, mlen := protowire.ConsumeVarint(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.ChunkMs = uint32(v)
			b = b[mlen:]
		case 4:
			v, mlen := protowire.ConsumeVarint(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.SampleRate = uint32(v)
			b = b[mlen:]
		default:
			mlen := protowire.ConsumeFieldValue(num, typ, b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			b = b[mlen:]
		}
	}
	return nil
}

type StartSession struct {
	Language      string
	StartLanguage string
}

func (m *StartSession) marshalWire() []byte {
	var b []byte
	if m.Language != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, m.Language)
	}
	if m.StartLanguage != "" {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, m.StartLanguage)
	}
	return b
}

type AudioChunk struct {
	PcmS16LE []byte
}

func (m *AudioChunk) marshalWire() []byte {
	var b []byte
	if len(m.PcmS16LE) > 0 {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, m.PcmS16LE)
	}
	return b
}

type RecognizeRequest struct {
	Start  *StartSession
	Audio  *AudioChunk
	Finish bool
	Cancel bool
}

func (m *RecognizeRequest) marshalWire() []byte {
	var b []byte
	switch {
	case m.Start != nil:
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, m.Start.marshalWire())
	case m.Audio != nil:
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, m.Audio.marshalWire())
	case m.Finish:
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendBytes(b, nil)
	case m.Cancel:
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendBytes(b, nil)
	}
	return b
}

func (m *RecognizeRequest) unmarshalWire([]byte) error { return nil }

type PartialTranscript struct {
	Committed string
	Open      string
	Tail      string
}

func (m *PartialTranscript) unmarshalWire(b []byte) error {
	*m = PartialTranscript{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, mlen := protowire.ConsumeString(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.Committed = v
			b = b[mlen:]
		case 2:
			v, mlen := protowire.ConsumeString(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.Open = v
			b = b[mlen:]
		case 3:
			v, mlen := protowire.ConsumeString(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.Tail = v
			b = b[mlen:]
		default:
			mlen := protowire.ConsumeFieldValue(num, typ, b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			b = b[mlen:]
		}
	}
	return nil
}

type CommitTranscript struct {
	Text string
}

func (m *CommitTranscript) unmarshalWire(b []byte) error {
	*m = CommitTranscript{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		if num == 1 {
			v, mlen := protowire.ConsumeString(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.Text = v
			b = b[mlen:]
			continue
		}
		mlen := protowire.ConsumeFieldValue(num, typ, b)
		if mlen < 0 {
			return protowire.ParseError(mlen)
		}
		b = b[mlen:]
	}
	return nil
}

type EndOfUtterance struct {
	AtSeconds   float32
	Probability float32
}

func (m *EndOfUtterance) unmarshalWire(b []byte) error {
	*m = EndOfUtterance{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, mlen := protowire.ConsumeFixed32(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.AtSeconds = math.Float32frombits(v)
			b = b[mlen:]
		case 2:
			v, mlen := protowire.ConsumeFixed32(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.Probability = math.Float32frombits(v)
			b = b[mlen:]
		default:
			mlen := protowire.ConsumeFieldValue(num, typ, b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			b = b[mlen:]
		}
	}
	return nil
}

type LanguageDetected struct {
	Language string
}

func (m *LanguageDetected) unmarshalWire(b []byte) error {
	*m = LanguageDetected{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		if num == 1 {
			v, mlen := protowire.ConsumeString(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.Language = v
			b = b[mlen:]
			continue
		}
		mlen := protowire.ConsumeFieldValue(num, typ, b)
		if mlen < 0 {
			return protowire.ParseError(mlen)
		}
		b = b[mlen:]
	}
	return nil
}

type FinalTranscript struct {
	Text string
}

func (m *FinalTranscript) unmarshalWire(b []byte) error {
	*m = FinalTranscript{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		if num == 1 {
			v, mlen := protowire.ConsumeString(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.Text = v
			b = b[mlen:]
			continue
		}
		mlen := protowire.ConsumeFieldValue(num, typ, b)
		if mlen < 0 {
			return protowire.ParseError(mlen)
		}
		b = b[mlen:]
	}
	return nil
}

type RecognizeEvent struct {
	Partial  *PartialTranscript
	Commit   *CommitTranscript
	Eou      *EndOfUtterance
	Language *LanguageDetected
	Final    *FinalTranscript
}

func (m *RecognizeEvent) marshalWire() []byte { return nil }

func (m *RecognizeEvent) unmarshalWire(b []byte) error {
	*m = RecognizeEvent{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			raw, mlen := protowire.ConsumeBytes(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			var v PartialTranscript
			if err := v.unmarshalWire(raw); err != nil {
				return err
			}
			m.Partial = &v
			b = b[mlen:]
		case 2:
			raw, mlen := protowire.ConsumeBytes(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			var v CommitTranscript
			if err := v.unmarshalWire(raw); err != nil {
				return err
			}
			m.Commit = &v
			b = b[mlen:]
		case 3:
			raw, mlen := protowire.ConsumeBytes(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			var v EndOfUtterance
			if err := v.unmarshalWire(raw); err != nil {
				return err
			}
			m.Eou = &v
			b = b[mlen:]
		case 4:
			raw, mlen := protowire.ConsumeBytes(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			var v LanguageDetected
			if err := v.unmarshalWire(raw); err != nil {
				return err
			}
			m.Language = &v
			b = b[mlen:]
		case 5:
			raw, mlen := protowire.ConsumeBytes(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			var v FinalTranscript
			if err := v.unmarshalWire(raw); err != nil {
				return err
			}
			m.Final = &v
			b = b[mlen:]
		default:
			mlen := protowire.ConsumeFieldValue(num, typ, b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			b = b[mlen:]
		}
	}
	return nil
}

type TranscribeRequest struct {
	PcmS16LE      []byte
	Language      string
	StartLanguage string
}

func (m *TranscribeRequest) marshalWire() []byte {
	var b []byte
	if len(m.PcmS16LE) > 0 {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, m.PcmS16LE)
	}
	if m.Language != "" {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, m.Language)
	}
	if m.StartLanguage != "" {
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendString(b, m.StartLanguage)
	}
	return b
}

func (*TranscribeRequest) unmarshalWire([]byte) error { return nil }

type TranscribeResponse struct {
	Text     string
	Language string
}

func (*TranscribeResponse) marshalWire() []byte { return nil }

func (m *TranscribeResponse) unmarshalWire(b []byte) error {
	*m = TranscribeResponse{}
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			v, mlen := protowire.ConsumeString(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.Text = v
			b = b[mlen:]
		case 2:
			v, mlen := protowire.ConsumeString(b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			m.Language = v
			b = b[mlen:]
		default:
			mlen := protowire.ConsumeFieldValue(num, typ, b)
			if mlen < 0 {
				return protowire.ParseError(mlen)
			}
			b = b[mlen:]
		}
	}
	return nil
}

type ShutdownRequest struct{}

func (*ShutdownRequest) marshalWire() []byte      { return nil }
func (*ShutdownRequest) unmarshalWire([]byte) error { return nil }

type ShutdownResponse struct{}

func (*ShutdownResponse) marshalWire() []byte      { return nil }
func (*ShutdownResponse) unmarshalWire([]byte) error { return nil }

type AsrServiceClient interface {
	Status(ctx context.Context, in *StatusRequest, opts ...grpc.CallOption) (*StatusResponse, error)
	StreamRecognize(ctx context.Context, opts ...grpc.CallOption) (StreamRecognizeClient, error)
	Transcribe(ctx context.Context, in *TranscribeRequest, opts ...grpc.CallOption) (*TranscribeResponse, error)
	Shutdown(ctx context.Context, in *ShutdownRequest, opts ...grpc.CallOption) (*ShutdownResponse, error)
}

type StreamRecognizeClient interface {
	Send(*RecognizeRequest) error
	Recv() (*RecognizeEvent, error)
	CloseSend() error
}

type asrServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewAsrServiceClient(cc grpc.ClientConnInterface) AsrServiceClient {
	return &asrServiceClient{cc: cc}
}

func (c *asrServiceClient) Status(ctx context.Context, in *StatusRequest, opts ...grpc.CallOption) (*StatusResponse, error) {
	out := new(StatusResponse)
	err := c.cc.Invoke(ctx, methodStatus, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *asrServiceClient) StreamRecognize(ctx context.Context, opts ...grpc.CallOption) (StreamRecognizeClient, error) {
	desc := &grpc.StreamDesc{
		StreamName:    "StreamRecognize",
		ServerStreams: true,
		ClientStreams: true,
	}
	stream, err := c.cc.NewStream(ctx, desc, methodStream, opts...)
	if err != nil {
		return nil, err
	}
	return &streamRecognizeClient{ClientStream: stream}, nil
}

func (c *asrServiceClient) Transcribe(ctx context.Context, in *TranscribeRequest, opts ...grpc.CallOption) (*TranscribeResponse, error) {
	out := new(TranscribeResponse)
	err := c.cc.Invoke(ctx, methodTranscribe, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *asrServiceClient) Shutdown(ctx context.Context, in *ShutdownRequest, opts ...grpc.CallOption) (*ShutdownResponse, error) {
	out := new(ShutdownResponse)
	err := c.cc.Invoke(ctx, methodShutdown, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type streamRecognizeClient struct {
	grpc.ClientStream
}

func (x *streamRecognizeClient) Send(m *RecognizeRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *streamRecognizeClient) Recv() (*RecognizeEvent, error) {
	m := new(RecognizeEvent)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}
