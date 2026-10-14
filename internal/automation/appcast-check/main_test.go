package main

import "testing"

func TestAppcastSignatureAttribute(t *testing.T) {
	for _, test := range []struct {
		name, xml string
		valid     bool
	}{
		{"signed", `<rss xmlns:s="http://www.andymatuschak.org/xml-namespaces/sparkle"><channel><item><enclosure s:edSignature="signature"/></item></channel></rss>`, true},
		{"unsigned", `<rss><channel><item><enclosure/></item></channel></rss>`, false},
		{"wrong namespace", `<rss xmlns:s="https://other.invalid"><channel><item><enclosure s:edSignature="signature"/></item></channel></rss>`, false},
		{"no enclosure", `<rss><channel><item/></channel></rss>`, false},
		{"first unsigned", `<rss xmlns:s="http://www.andymatuschak.org/xml-namespaces/sparkle"><channel><item><enclosure/></item><item><enclosure s:edSignature="signature"/></item></channel></rss>`, false},
		{"malformed", `<rss>`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := check([]byte(test.xml))
			if (err == nil) != test.valid {
				t.Fatal(err)
			}
		})
	}
}
