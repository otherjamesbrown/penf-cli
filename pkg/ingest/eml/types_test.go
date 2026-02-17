package eml

import (
	"reflect"
	"testing"
)

// TestAllParticipantPairs tests that email+displayName pairs are returned from parsed email headers.
// This test currently FAILS because AllParticipantPairs() does not exist yet.
func TestAllParticipantPairs(t *testing.T) {
	tests := []struct {
		name  string
		email ParsedEmail
		want  []Address
	}{
		{
			name: "all participants with display names",
			email: ParsedEmail{
				From: Address{Email: "sender@example.com", Name: "Sender Name"},
				To: []Address{
					{Email: "recipient1@example.com", Name: "Recipient One"},
					{Email: "recipient2@example.com", Name: "Recipient Two"},
				},
				Cc: []Address{
					{Email: "cc1@example.com", Name: "CC Person"},
				},
			},
			want: []Address{
				{Email: "sender@example.com", Name: "Sender Name"},
				{Email: "recipient1@example.com", Name: "Recipient One"},
				{Email: "recipient2@example.com", Name: "Recipient Two"},
				{Email: "cc1@example.com", Name: "CC Person"},
			},
		},
		{
			name: "some participants without display names",
			email: ParsedEmail{
				From: Address{Email: "sender@example.com", Name: ""},
				To: []Address{
					{Email: "recipient@example.com", Name: "Recipient Name"},
				},
			},
			want: []Address{
				{Email: "sender@example.com", Name: ""},
				{Email: "recipient@example.com", Name: "Recipient Name"},
			},
		},
		{
			name: "display name with Last, First format",
			email: ParsedEmail{
				From: Address{Email: "koslakov@akamai.com", Name: "Oslakovic, Keith"},
				To: []Address{
					{Email: "recipient@example.com", Name: "Smith, John"},
				},
			},
			want: []Address{
				{Email: "koslakov@akamai.com", Name: "Oslakovic, Keith"},
				{Email: "recipient@example.com", Name: "Smith, John"},
			},
		},
		{
			name: "empty email addresses skipped",
			email: ParsedEmail{
				From: Address{Email: "sender@example.com", Name: "Sender"},
				To: []Address{
					{Email: "", Name: "No Email"},
					{Email: "valid@example.com", Name: "Valid"},
				},
			},
			want: []Address{
				{Email: "sender@example.com", Name: "Sender"},
				{Email: "valid@example.com", Name: "Valid"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.email.AllParticipantPairs()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AllParticipantPairs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllParticipantEmails_WithNames(t *testing.T) {
	email := ParsedEmail{
		From: Address{Email: "sender@example.com", Name: "Sender Name"},
		To: []Address{
			{Email: "recipient1@example.com", Name: "Recipient One"},
			{Email: "recipient2@example.com", Name: "Recipient Two"},
		},
		Cc: []Address{
			{Email: "cc@example.com", Name: "CC Person"},
		},
		Bcc: []Address{
			{Email: "bcc@example.com", Name: "BCC Person"},
		},
	}

	want := []string{
		"sender@example.com",
		"recipient1@example.com",
		"recipient2@example.com",
		"cc@example.com",
		"bcc@example.com",
	}

	got := email.AllParticipantEmails()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AllParticipantEmails() = %v, want %v", got, want)
	}
}
