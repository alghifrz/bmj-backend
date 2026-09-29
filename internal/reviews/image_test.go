package reviews

import "testing"

func TestSniffReviewImage(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantType  string
		wantExt   string
		wantMatch bool
	}{
		{name: "jpeg", data: []byte{0xFF, 0xD8, 0xFF, 0x00}, wantType: "image/jpeg", wantExt: "jpg", wantMatch: true},
		{name: "png", data: []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}, wantType: "image/png", wantExt: "png", wantMatch: true},
		{name: "text", data: []byte("not-an-image"), wantMatch: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contentType, extension, ok := sniffReviewImage(test.data)
			if ok != test.wantMatch || contentType != test.wantType || extension != test.wantExt {
				t.Fatalf("sniff = %s %s %v", contentType, extension, ok)
			}
		})
	}
}
