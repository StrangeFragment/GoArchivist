package mail

import "testing"

func TestDecodeModifiedUTF7(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "INBOX", want: "INBOX"},
		{input: "&-", want: "&"},
		{input: "&BB4EQgQ,BEAEMAQyBDsENQQ9BD0ESwQ1-", want: "Отправленные"},
		{input: "Archive/&BB4EQgQ,BEAEMAQyBDsENQQ9BD0ESwQ1-", want: "Archive/Отправленные"},
		{input: "Malformed &BB4EQgQ-", want: "Malformed &BB4EQgQ-"},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			if got := DecodeModifiedUTF7(test.input); got != test.want {
				t.Fatalf("DecodeModifiedUTF7(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestDisplayMailboxName(t *testing.T) {
	if got := DisplayMailboxName("inbox"); got != "Входящие" {
		t.Fatalf("DisplayMailboxName(inbox) = %q, want Входящие", got)
	}
}
