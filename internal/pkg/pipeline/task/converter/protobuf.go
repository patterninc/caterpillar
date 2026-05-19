package converter

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3client "github.com/patterninc/caterpillar/internal/pkg/pipeline/task/file/s3_client"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	protobufS3Scheme      = "s3://"
	protobufDefaultRegion = "us-west-2"
)

type protobuf struct {
	DescriptorPath  string `yaml:"descriptor_path" json:"descriptor_path"`
	MessageName     string `yaml:"message_name" json:"message_name"`
	Region          string `yaml:"region,omitempty" json:"region,omitempty"`
	UseProtoNames   bool   `yaml:"use_proto_names,omitempty" json:"use_proto_names,omitempty"`
	EmitUnpopulated bool   `yaml:"emit_unpopulated,omitempty" json:"emit_unpopulated,omitempty"`

	once   sync.Once
	md     protoreflect.MessageDescriptor
	loaded error
}

func (c *protobuf) load() error {
	c.once.Do(func() {
		if c.DescriptorPath == "" || c.MessageName == "" {
			c.loaded = fmt.Errorf("protobuf converter requires descriptor_path and message_name")
			return
		}

		raw, err := c.readDescriptor()
		if err != nil {
			c.loaded = fmt.Errorf("read descriptor: %w", err)
			return
		}

		fds := &descriptorpb.FileDescriptorSet{}
		if err := proto.Unmarshal(raw, fds); err != nil {
			c.loaded = fmt.Errorf("unmarshal FileDescriptorSet: %w", err)
			return
		}

		files, err := protodesc.NewFiles(fds)
		if err != nil {
			c.loaded = fmt.Errorf("build descriptor registry: %w", err)
			return
		}

		desc, err := files.FindDescriptorByName(protoreflect.FullName(c.MessageName))
		if err != nil {
			c.loaded = fmt.Errorf("find message %q: %w", c.MessageName, err)
			return
		}

		md, ok := desc.(protoreflect.MessageDescriptor)
		if !ok {
			c.loaded = fmt.Errorf("%q is not a message descriptor", c.MessageName)
			return
		}
		c.md = md
	})
	return c.loaded
}

func (c *protobuf) readDescriptor() ([]byte, error) {
	if !strings.HasPrefix(c.DescriptorPath, protobufS3Scheme) {
		return os.ReadFile(c.DescriptorPath)
	}

	region := c.Region
	if region == "" {
		region = protobufDefaultRegion
	}

	ctx := context.Background()
	client, err := s3client.New(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("create s3 client: %w", err)
	}

	bucket, key, err := s3client.ParseURI(c.DescriptorPath)
	if err != nil {
		return nil, err
	}

	out, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return nil, fmt.Errorf("s3 GetObject %s: %w", c.DescriptorPath, err)
	}
	defer out.Body.Close()

	return io.ReadAll(out.Body)
}

func (c *protobuf) convert(data []byte, _ string) ([]converterOutput, error) {
	if err := c.load(); err != nil {
		return nil, err
	}

	msg := dynamicpb.NewMessage(c.md)
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("unmarshal protobuf message: %w", err)
	}

	marshaler := protojson.MarshalOptions{
		UseProtoNames:   c.UseProtoNames,
		EmitUnpopulated: c.EmitUnpopulated,
		Resolver:        protoregistry.GlobalTypes,
	}
	jsonData, err := marshaler.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal to JSON: %w", err)
	}

	return []converterOutput{{Data: jsonData}}, nil
}
