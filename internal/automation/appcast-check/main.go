// Appcast check verifies Sparkle supplied the updater signature attribute.
package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
)

func check(raw []byte) error {
	var feed struct {
		Channel struct {
			Items []struct {
				Enclosure *struct {
					Signature string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle edSignature,attr"`
				} `xml:"enclosure"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(raw, &feed); err != nil {
		return err
	}
	if len(feed.Channel.Items) == 0 || feed.Channel.Items[0].Enclosure == nil || feed.Channel.Items[0].Enclosure.Signature == "" {
		return errors.New("sparkle did not sign the update; check the private key matches SUPublicEDKey")
	}
	return nil
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: appcast-check APPCAST.xml")
		os.Exit(1)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err == nil {
		err = check(raw)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
