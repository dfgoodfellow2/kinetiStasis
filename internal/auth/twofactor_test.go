package auth

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

// TestGenerateTOTPKey proves the enrollment secret is well formed and the URL
// points at the DietTracker issuer for the account email.
func TestGenerateTOTPKey(t *testing.T) {
	secret, otpauthURL, err := GenerateTOTPKey("alice@example.com")
	if err != nil {
		t.Fatalf("GenerateTOTPKey: %v", err)
	}
	if secret == "" {
		t.Fatal("empty secret")
	}
	if !strings.HasPrefix(otpauthURL, "otpauth://totp/") {
		t.Fatalf("otpauth URL = %q, want otpauth://totp/ prefix", otpauthURL)
	}
	u, err := url.Parse(otpauthURL)
	if err != nil {
		t.Fatalf("parse otpauth URL: %v", err)
	}
	if got := u.Query().Get("issuer"); got != TOTPIssuer {
		t.Fatalf("issuer = %q, want %q", got, TOTPIssuer)
	}
	if got := u.Query().Get("secret"); got != secret {
		t.Fatalf("URL secret = %q, want generated secret", got)
	}
	if !strings.Contains(u.Path, "alice@example.com") {
		t.Fatalf("URL path = %q, want account email", u.Path)
	}

	if _, _, err := GenerateTOTPKey("   "); err == nil {
		t.Fatal("empty account should fail")
	}
}

// TestValidateTOTPWindow pins the ±1 period window: the current code and both
// adjacent 30-second codes verify, while a code from three periods ago does
// not.
func TestValidateTOTPWindow(t *testing.T) {
	secret, _, err := GenerateTOTPKey("bob@example.com")
	if err != nil {
		t.Fatalf("GenerateTOTPKey: %v", err)
	}
	now := time.Now().UTC()

	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"current", now, true},
		{"previous period (skew -1)", now.Add(-TOTPPeriodSeconds * time.Second), true},
		{"next period (skew +1)", now.Add(TOTPPeriodSeconds * time.Second), true},
		{"three periods ago", now.Add(-3 * TOTPPeriodSeconds * time.Second), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, err := totp.GenerateCode(secret, tc.at)
			if err != nil {
				t.Fatalf("GenerateCode: %v", err)
			}
			if got := ValidateTOTP(secret, code); got != tc.want {
				t.Fatalf("ValidateTOTP(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestValidateTOTPRejectsGarbage keeps malformed input from ever validating.
func TestValidateTOTPRejectsGarbage(t *testing.T) {
	secret, _, err := GenerateTOTPKey("carol@example.com")
	if err != nil {
		t.Fatalf("GenerateTOTPKey: %v", err)
	}

	for _, code := range []string{"", "123", "abcdef", "000000"} {
		if ValidateTOTP(secret, code) {
			t.Fatalf("ValidateTOTP accepted %q", code)
		}
	}
	if ValidateTOTP("", "123456") {
		t.Fatal("ValidateTOTP accepted an empty secret")
	}
}

// TestTOTPURL confirms the QR URL builder uses the supplied secret (not a
// freshly generated one) and the expected defaults.
func TestTOTPURL(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	raw, err := TOTPURL(secret, "dave@example.com")
	if err != nil {
		t.Fatalf("TOTPURL: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()
	if q.Get("secret") != secret {
		t.Fatalf("secret = %q, want %q", q.Get("secret"), secret)
	}
	if q.Get("issuer") != TOTPIssuer || q.Get("algorithm") != "SHA1" || q.Get("digits") != "6" || q.Get("period") != "30" {
		t.Fatalf("unexpected otpauth parameters: %v", q)
	}
	if !strings.Contains(u.Path, "dave@example.com") {
		t.Fatalf("path = %q, want account email", u.Path)
	}

	if _, err := TOTPURL("", "dave@example.com"); err == nil {
		t.Fatal("empty secret should fail")
	}
	if _, err := TOTPURL(secret, ""); err == nil {
		t.Fatal("empty account should fail")
	}
}

// TestGenerateRecoveryCodes pins count, length, alphabet and uniqueness.
func TestGenerateRecoveryCodes(t *testing.T) {
	codes, err := GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes: %v", err)
	}
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("code count = %d, want %d", len(codes), RecoveryCodeCount)
	}
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if len(code) != RecoveryCodeLength {
			t.Fatalf("code %q length = %d, want %d", code, len(code), RecoveryCodeLength)
		}
		for _, r := range code {
			if !strings.ContainsRune(recoveryAlphabet, r) {
				t.Fatalf("code %q contains %q outside the invite alphabet", code, r)
			}
		}
		if _, dup := seen[code]; dup {
			t.Fatalf("duplicate recovery code %q", code)
		}
		seen[code] = struct{}{}
	}
}

// TestHashRecoveryCodeNormalises proves hashing is stable, strips surrounding
// whitespace, is case-insensitive, hides the plaintext, and distinguishes
// different codes.
func TestHashRecoveryCodeNormalises(t *testing.T) {
	const code = "ABCD2345EFGH"
	h := HashRecoveryCode(code)
	if h == "" || h == code {
		t.Fatalf("HashRecoveryCode did not produce a distinct digest: %q", h)
	}
	if len(h) != 64 {
		t.Fatalf("hash length = %d, want 64 hex chars", len(h))
	}
	if got := HashRecoveryCode("  " + strings.ToLower(code) + "  "); got != h {
		t.Fatalf("normalised hash = %q, want %q", got, h)
	}
	if other := HashRecoveryCode("ABCD2345EFGI"); other == h {
		t.Fatal("different codes produced the same hash")
	}
}

// TestOpaqueTokens confirms challenge/device tokens are random and that
// hashing is deterministic and hides the token.
func TestOpaqueTokens(t *testing.T) {
	a, err := GenerateChallengeToken()
	if err != nil {
		t.Fatalf("GenerateChallengeToken: %v", err)
	}
	b, err := GenerateChallengeToken()
	if err != nil {
		t.Fatalf("GenerateChallengeToken: %v", err)
	}
	if a == "" || a == b {
		t.Fatalf("challenge tokens not unique: %q / %q", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("challenge token length = %d, want 64 hex chars", len(a))
	}
	if HashOpaqueToken(a) != HashOpaqueToken(a) {
		t.Fatal("HashOpaqueToken is not deterministic")
	}
	if HashOpaqueToken(a) == a {
		t.Fatal("HashOpaqueToken returned the token itself")
	}

	d, err := GenerateDeviceToken()
	if err != nil {
		t.Fatalf("GenerateDeviceToken: %v", err)
	}
	if d == "" || d == a {
		t.Fatal("device token was empty or reused a challenge token")
	}
}

// TestTwoFactorConstants keeps the documented contract from drifting.
func TestTwoFactorConstants(t *testing.T) {
	if MFAChallengeTTL != 5*time.Minute {
		t.Fatalf("MFAChallengeTTL = %v, want 5m", MFAChallengeTTL)
	}
	if TrustedDeviceTTL != 30*24*time.Hour {
		t.Fatalf("TrustedDeviceTTL = %v, want 30d", TrustedDeviceTTL)
	}
	if RecoveryCodeCount != 10 || RecoveryCodeLength != 12 {
		t.Fatalf("recovery constants = %d/%d, want 10/12", RecoveryCodeCount, RecoveryCodeLength)
	}
}
