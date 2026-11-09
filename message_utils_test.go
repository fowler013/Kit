package main

import "testing"

func TestStripMentionForUser(t *testing.T) {
	got := stripMentionForUser("  hello <@U123> there  ", "U123")
	want := "hello there"
	if got != want {
		t.Fatalf("stripMentionForUser() = %q, want %q", got, want)
	}

	got = stripMentionForUser("<@!U123> hello", "U123")
	want = "hello"
	if got != want {
		t.Fatalf("stripMentionForUser() = %q, want %q", got, want)
	}
}

func TestRemoveBotMention(t *testing.T) {
	got := removeBotMention("<@U123> hello <@U456> world")
	want := "hello world"
	if got != want {
		t.Fatalf("removeBotMention() = %q, want %q", got, want)
	}
}
