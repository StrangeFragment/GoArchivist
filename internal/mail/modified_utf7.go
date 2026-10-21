package mail

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// DecodeModifiedUTF7 decodes an IMAP mailbox name from Modified UTF-7.
// Malformed encoded sections are preserved verbatim to keep the server name
// available for display and future protocol commands.
func DecodeModifiedUTF7(value string) string {
	var decoded strings.Builder
	for i := 0; i < len(value); {
		if value[i] != '&' {
			decoded.WriteByte(value[i])
			i++
			continue
		}

		relativeEnd := strings.IndexByte(value[i:], '-')
		if relativeEnd < 0 {
			decoded.WriteByte(value[i])
			i++
			continue
		}
		end := i + relativeEnd
		if end == i+1 {
			decoded.WriteByte('&')
			i = end + 1
			continue
		}

		encoded := strings.ReplaceAll(value[i+1:end], ",", "/")
		if remainder := len(encoded) % 4; remainder != 0 {
			encoded += strings.Repeat("=", 4-remainder)
		}
		bytes, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(bytes)%2 != 0 {
			decoded.WriteString(value[i : end+1])
			i = end + 1
			continue
		}

		units := make([]uint16, len(bytes)/2)
		for j := range units {
			units[j] = binary.BigEndian.Uint16(bytes[j*2 : j*2+2])
		}
		decoded.WriteString(string(utf16.Decode(units)))
		i = end + 1
	}
	return decoded.String()
}

// DisplayMailboxName returns a readable label without changing the protocol name.
func DisplayMailboxName(name string) string {
	if strings.EqualFold(name, "INBOX") {
		return "Входящие"
	}
	return DecodeModifiedUTF7(name)
}
