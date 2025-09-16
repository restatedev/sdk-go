package rand

import (
	"encoding/hex"
	"testing"
)

func TestUint64(t *testing.T) {
	id, err := hex.DecodeString("f311f1fdcb9863f0018bd3400ecd7d69b547204e776218b2")
	if err != nil {
		t.Fatal(err)
	}
	rand := NewFromInvocationId(id)

	expected := []uint64{
		6541268553928124324,
		1632128201851599825,
		3999496359968271420,
		9099219592091638755,
		2609122094717920550,
		16569362788292807660,
		14955958648458255954,
		15581072429430901841,
		4951852598761288088,
		2380816196140950843,
	}

	for _, e := range expected {
		if found := rand.Uint64(); e != found {
			t.Fatalf("Unexpected uint64 %d, expected %d", found, e)
		}
	}
}

func TestFloat64(t *testing.T) {
	source := &source{state: [4]uint64{1, 2, 3, 4}}
	rand := &rand{source}

	expected := []float64{
		4.656612984099695e-9, 6.519269457605503e-9, 0.39843750651926946,
		0.3986824029416509, 0.5822761557370711, 0.2997488042907357,
		0.5336032865255543, 0.36335061693258097, 0.5968067925950846,
		0.18570456306457928,
	}

	for _, e := range expected {
		if found := rand.Float64(); e != found {
			t.Fatalf("Unexpected float64 %v, expected %v", found, e)
		}
	}
}

func TestUUID(t *testing.T) {
	source := &source{state: [4]uint64{1, 2, 3, 4}}
	rand := &rand{source}

	expected := []string{
		"01008002-0000-4000-a700-800300000000",
		"67008003-00c0-4c00-b200-449901c20c00",
		"cd33c49a-01a2-4280-ba33-eecd8a97698a",
		"bd4a1533-4713-41c2-979e-167991a02bac",
		"d83f078f-0a19-43db-a092-22b24af10591",
		"677c91f7-146e-4769-a4fd-df3793e717e8",
		"f15179b2-f220-4427-8d90-7b5437d9828d",
		"9e97720f-42b8-4d09-a449-914cf221df26",
		"09d0a109-6f11-4ef9-93fa-f013d0ad3808",
		"41eb0e0c-41c9-4828-85d0-59fb901b4df4",
	}

	for _, e := range expected {
		if found := rand.UUID().String(); e != found {
			t.Fatalf("Unexpected uuid %s, expected %s", found, e)
		}
	}
}

func TestUUIDFromSeed(t *testing.T) {
	rand := NewFromSeed(1)

	expected := []string{
		"9bc2036f-7fd0-45cf-8de0-3f96324142bf",
		"20f5aa57-577d-4319-9656-cd059f1108bf",
		"a46f1886-4b18-472f-8523-20e7ca9f2997",
		"0715f408-95c7-43fc-a1f2-6303c9a5fe85",
		"d04b330d-b3e5-4a18-96ec-26f0c9136122",
		"49e6cfdc-f90e-4eeb-b3ff-6c9fddaeef57",
		"d6407669-d5e2-4a12-8950-50ee4e3a0365",
		"ba884ad5-3e45-4916-bba8-28f4a85a0628",
		"a21d045f-1647-408e-b32e-f7a4d9321079",
		"9eed3928-5482-48f5-8a0f-8040b1de6aa4",
	}

	for _, e := range expected {
		if found := rand.UUID().String(); e != found {
			t.Fatalf("Unexpected uuid %s, expected %s", found, e)
		}
	}
}
