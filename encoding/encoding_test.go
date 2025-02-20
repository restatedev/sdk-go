package encoding

import (
	"testing"
)

func TestVoid(t *testing.T) {
	codecs := map[string]Codec{
		"json":      JSONCodec,
		"proto":     ProtoCodec,
		"protojson": ProtoJSONCodec,
		"binary":    BinaryCodec,
	}
	for name, codec := range codecs {
		t.Run(name, func(t *testing.T) {
			bytes, err := Marshal(codec, Void{})
			if err != nil {
				t.Fatal(err)
			}

			if bytes != nil {
				t.Fatalf("expected bytes to be nil, found %v", bytes)
			}

			if err := Unmarshal(codec, []byte{1, 2, 3}, &Void{}); err != nil {
				t.Fatal(err)
			}

			if err := Unmarshal(codec, []byte{1, 2, 3}, Void{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
