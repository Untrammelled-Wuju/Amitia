package serviceauth

import "testing"

func TestTokenStableAndScoped(t *testing.T) {
	a, err := Token("com.example/test", "service-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Token("com.example/test", "service-a")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Token("com.example/test", "service-b")
	if err != nil {
		t.Fatal(err)
	}
	if a == "" || a != b {
		t.Fatalf("expected stable non-empty token, got %q and %q", a, b)
	}
	if a == c {
		t.Fatal("expected module-scoped tokens to differ")
	}
}
