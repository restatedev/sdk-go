package internal

import (
	_ "embed"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

var (
	requestTypeName  = "RpcRequest"
	responseTypeName = "RpcResponse"
)

//go:embed dynrpc.binbp
var dynRpcBytes []byte

// NewDynRpcDescriptorSet creates a new DynRpcDescriptorSet
func NewDynRpcDescriptorSet() *DynRpcDescriptorSet {
	var ds descriptorpb.FileDescriptorSet
	err := proto.Unmarshal(dynRpcBytes, &ds)
	if err != nil {
		panic(fmt.Sprintf("invalid file descriptor set: %s", err))
	}

	return &DynRpcDescriptorSet{&ds}
}

// DynRpcService wrapper around ServiceDescriptorProto for easier manipulation
type DynRpcService struct {
	*descriptorpb.ServiceDescriptorProto
}

func (s *DynRpcService) AddHandler(name string) {
	s.Method = append(s.Method, &descriptorpb.MethodDescriptorProto{
		Name:       &name,
		InputType:  &requestTypeName,
		OutputType: &responseTypeName,
		// TODO: check options and others
	})
}

// DynRpcDescriptorSet wrapper around FileDescriptorSet for easier manipulation
type DynRpcDescriptorSet struct {
	*descriptorpb.FileDescriptorSet
}

// Inner returns the inner *descriptorpb.FileDescriptorSet
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

// AddKeyedService creates a new Keyed service from the KeyedRpcService template
func (d *DynRpcDescriptorSet) AddKeyedService(name string) (*DynRpcService, error) {
	return d.addService(name, 0)
}

// AddUnKeyedService creates a new Un-Keyed service from the RpcService template
func (d *DynRpcDescriptorSet) AddUnKeyedService(name string) (*DynRpcService, error) {
	return d.addService(name, 1)
}

func (d *DynRpcDescriptorSet) addService(name string, from int) (*DynRpcService, error) {
	file, err := d.getDynRpcFile()
	if err != nil {
		return nil, err
	}

	// unkeyed service is always number one
	service, err := deepCopy(file.Service[from])
	if err != nil {
		return nil, err
	}

	service.Name = &name
	// clean up services
	service.Method = []*descriptorpb.MethodDescriptorProto{}

	file.Service = append(file.Service, service)

	return &DynRpcService{service}, nil
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
