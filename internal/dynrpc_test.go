package internal

import (
	"fmt"
	"testing"
)

func TestDynRpc(t *testing.T) {

	ds := NewDynRpcDescriptorSet()

	//fmt.Println(len(ds.File))
	//	require.Len(t, ds.File, 2)

	for _, file := range ds.File {

		fmt.Println(*file.Name)
		fmt.Printf("dep: %+v\n", file.Dependency)
		for _, service := range file.Service {
			fmt.Println("  - service: ", *service.Name)
		}
	}
}
