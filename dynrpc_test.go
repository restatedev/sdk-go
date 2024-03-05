package restate

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

// func TestDeepCopy(t *testing.T) {
// 	type T struct {
// 		name  string
// 		array []int
// 	}

// 	x := T{
// 		name:  "test",
// 		array: []int{0, 1},
// 	}

// 	y := DeepCopy(&x)

// 	require.Equal(t, x.name, y.name)
// 	require.Equal(t, x.array, y.array)

// }
