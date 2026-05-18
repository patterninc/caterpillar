package converter

import (
	"fmt"
	"os"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type protobuf struct {
	DescriptorPath  string `yaml:"descriptor_path" json:"descriptor_path"`
	MessageName     string `yaml:"message_name" json:"message_name"`
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

		raw, err := os.ReadFile(c.DescriptorPath)
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
