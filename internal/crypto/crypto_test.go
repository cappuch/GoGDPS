package crypto

import "testing"

func TestXORCipherRoundTrip(t *testing.T) {
	plain := "testpassword"
	encoded := XORCipher(plain, 37526)
	decoded := XORCipher(encoded, 37526)
	if decoded != plain {
		t.Fatalf("expected %q, got %q", plain, decoded)
	}
}

func TestGenSoloShortString(t *testing.T) {
	got := GenSolo("hello")
	want := sha1Hex("hello" + hashSalt1)
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestGJP2FromPassword(t *testing.T) {
	got := GJP2FromPassword("mypass")
	if len(got) != 40 {
		t.Fatalf("expected sha1 hex length 40, got %d", len(got))
	}
}
