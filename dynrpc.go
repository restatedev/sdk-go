package restate

import (
	_ "embed"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

//go:embed generated/dynrpc.binbp
var dynRpcBytes []byte

// New makes sure we have a new instance every time it's called
func NewDynRpcDescriptorSet() *DynRpcDescriptorSet {
	var ds descriptorpb.FileDescriptorSet
	err := proto.Unmarshal(dynRpcBytes, &ds)
	if err != nil {
		panic(fmt.Sprintf("invalid file descriptor set: %s", err))
	}

	return &DynRpcDescriptorSet{&ds}
}

// DynRpcDescriptorSet wrapper around FileDescriptorSet for easy manipulation
type DynRpcDescriptorSet struct {
	*descriptorpb.FileDescriptorSet
}

func (d *DynRpcDescriptorSet) Inner() *descriptorpb.FileDescriptorSet {
	return d.FileDescriptorSet
}

func (d *DynRpcDescriptorSet) getDynRpcFile() (*descriptorpb.FileDescriptorProto, error) {
	// since it's always the last file may be we should use the index
	// but this is safer
	for _, file := range d.File {
		if file.Name != nil && *file.Name == "dynrpc/dynrpc.proto" {
			return file, nil
		}
	}

	return nil, fmt.Errorf("file descriptor for dynrpc not found")
}

func (d *DynRpcDescriptorSet) AddUnKeyedService(name string) error {
	file, err := d.getDynRpcFile()
	if err != nil {
		return err
	}

	// unkeyed service is always number one
	service, err := deepCopy(file.Service[1])
	if err != nil {
		return err
	}

	service.Name = &name
	service.Method = []*descriptorpb.MethodDescriptorProto{}
	file.Service = append(file.Service, service)

	return nil
}

func deepCopy(src *descriptorpb.ServiceDescriptorProto) (*descriptorpb.ServiceDescriptorProto, error) {
	bytes, err := proto.Marshal(src)
	if err != nil {
		return nil, err
	}

	var copy descriptorpb.ServiceDescriptorProto
	if err := proto.Unmarshal(bytes, &copy); err != nil {
		return nil, err
	}

	return &copy, nil

}
