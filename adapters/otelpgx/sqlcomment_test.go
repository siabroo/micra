package otelpgx

import "testing"

func TestEncode_AllFieldsAlphabetical(t *testing.T) {
	got := encode(commentFields{
		application: "auth-go",
		route:       "/auth.Auth/SignIn",
		traceparent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	})
	want := `/*application='auth-go',route='%2Fauth.Auth%2FSignIn',traceparent='00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01'*/`
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestEncode_SkipsEmptyFields(t *testing.T) {
	got := encode(commentFields{traceparent: "00-aaa-bbb-01"})
	want := `/*traceparent='00-aaa-bbb-01'*/`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEncode_AllEmpty_ReturnsEmpty(t *testing.T) {
	got := encode(commentFields{})
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
}

func TestEncode_URLEncodesSpecialChars(t *testing.T) {
	got := encode(commentFields{route: "/foo bar?x=1"})
	want := `/*route='%2Ffoo%20bar%3Fx%3D1'*/`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEncode_EscapesSingleQuoteInValue(t *testing.T) {
	got := encode(commentFields{application: "bob's-svc"})
	want := `/*application='bob%27s-svc'*/`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
