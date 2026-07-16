package password

import "testing"

func TestHashAndVerify(t *testing.T) {
	h, err := Hash("geoserver")
	if err != nil {
		t.Fatal(err)
	}
	if !Verify("geoserver", h) {
		t.Fatal("verify correct password = false")
	}
	if Verify("wrong", h) {
		t.Fatal("verify wrong password = true")
	}
	h2, _ := Hash("geoserver")
	if h == h2 {
		t.Fatal("hashes must be salted (identical output)")
	}
	if Verify("geoserver", "not-a-phc-hash") {
		t.Fatal("garbage hash must not verify")
	}
}
