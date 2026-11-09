package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// Two-factor authentication parameters. The TOTP window is the library
// default (±1 period, 30 s each), which tolerates clock drift without
// widening the brute-force surface beyond the rate-limited verify endpoint.
const (
	// TOTPIssuer is shown by authenticator apps next to the account email.
	TOTPIssuer = "DietTracker"

	// MFAChallengeTTL bounds how long a password-verified two-step login may
	// sit waiting for a code. The challenge is single-use.
	MFAChallengeTTL = 5 * time.Minute

	// TrustedDeviceTTL is the lifetime of a remember-device cookie and row.
	TrustedDeviceTTL = 30 * 24 * time.Hour

	// TOTPPeriodSeconds and TOTPDigits match Google Authenticator defaults.
	TOTPPeriodSeconds = 30
	TOTPDigits        = 6

	// Recovery codes are shown once. Twelve characters from the same
	// no-ambiguous alphabet as group/registration invites are readable aloud.
	RecoveryCodeCount  = 10
	RecoveryCodeLength = 12

	// TrustedDeviceCookieName carries the opaque remember-device token. Only
	// its SHA-256 hash is ever stored.
	TrustedDeviceCookieName = "dt_device"

	// recoveryAlphabet omits 0/O/1/I so codes survive being read aloud.
	recoveryAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// GenerateTOTPKey creates a fresh TOTP secret and its otpauth:// enrollment
// URL. The caller must NOT persist the secret until a live code confirms it.
func GenerateTOTPKey(account string) (secret, otpauthURL string, err error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return "", "", errors.New("account is required")
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: account,
		Period:      TOTPPeriodSeconds,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// TOTPURL rebuilds the otpauth:// enrollment URL for an already-generated
// secret. It is used by the QR image route so a pending secret never has to be
// persisted just to render a code.
func TOTPURL(secret, account string) (string, error) {
	secret = normalizeTOTPSecret(secret)
	account = strings.TrimSpace(account)
	if secret == "" {
		return "", errors.New("secret is required")
	}
	if account == "" {
		return "", errors.New("account is required")
	}

	values := url.Values{}
	values.Set("secret", secret)
	values.Set("issuer", TOTPIssuer)
	values.Set("period", strconv.Itoa(TOTPPeriodSeconds))
	values.Set("algorithm", "SHA1")
	values.Set("digits", strconv.Itoa(TOTPDigits))

	u := url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     "/" + TOTPIssuer + ":" + account,
		RawQuery: values.Encode(),
	}
	return u.String(), nil
}

// ValidateTOTP reports whether code is valid for secret right now. Skew is
// ±1 period, the same window authenticator apps use. Empty inputs are never
// valid and never panic.
func ValidateTOTP(secret, code string) bool {
	secret = normalizeTOTPSecret(secret)
	code = normalizeTOTPCode(code)
	if secret == "" || code == "" {
		return false
	}

	ok, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    TOTPPeriodSeconds,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && ok
}

// GenerateRecoveryCodes returns RecoveryCodeCount unused codes drawn from the
// no-ambiguous alphabet using crypto/rand.
func GenerateRecoveryCodes() ([]string, error) {
	codes := make([]string, RecoveryCodeCount)
	max := big.NewInt(int64(len(recoveryAlphabet)))
	for i := range codes {
		buf := make([]byte, RecoveryCodeLength)
		for j := range buf {
			n, err := rand.Int(rand.Reader, max)
			if err != nil {
				return nil, err
			}
			buf[j] = recoveryAlphabet[n.Int64()]
		}
		codes[i] = string(buf)
	}
	return codes, nil
}

// HashRecoveryCode normalises a user-typed code and returns its SHA-256 hex
// digest for storage or lookup. Normalisation makes the code forgiving of
// surrounding whitespace and lower-case entry without storing plaintext.
func HashRecoveryCode(code string) string {
	return hashOpaque(normalizeRecoveryCode(code))
}

// GenerateChallengeToken returns a 32-byte crypto/rand token for a two-step
// login challenge. Only its hash is stored.
func GenerateChallengeToken() (string, error) {
	return generateOpaqueToken(32)
}

// GenerateDeviceToken returns a 32-byte crypto/rand token for a trusted
// device cookie. Only its hash is stored.
func GenerateDeviceToken() (string, error) {
	return generateOpaqueToken(32)
}

// HashOpaqueToken returns the SHA-256 hex digest used for challenge and
// device-token storage. Tokens are never logged.
func HashOpaqueToken(token string) string {
	return hashOpaque(strings.TrimSpace(token))
}

// SetTrustedDeviceCookie writes the opaque remember-device token. It is
// httpOnly, SameSite=Lax and Secure in production, matching the session
// cookies so it cannot be read by scripts or sent on cross-site POSTs.
func SetTrustedDeviceCookie(w http.ResponseWriter, token string, isProd bool) {
	sameSite, secure := cookieFlags(isProd)
	http.SetCookie(w, &http.Cookie{
		Name:     TrustedDeviceCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(TrustedDeviceTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

// ClearTrustedDeviceCookie expires the remember-device cookie. Flags mirror
// SetTrustedDeviceCookie so browsers reliably overwrite it.
func ClearTrustedDeviceCookie(w http.ResponseWriter, isProd bool) {
	sameSite, secure := cookieFlags(isProd)
	http.SetCookie(w, &http.Cookie{
		Name:     TrustedDeviceCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func normalizeTOTPSecret(secret string) string {
	// Authenticator secrets are base32 and case-insensitive; users pasting a
	// secret sometimes include spaces. crypto/totp accepts the trimmed form.
	return strings.ToUpper(strings.Join(strings.Fields(secret), ""))
}

func normalizeTOTPCode(code string) string {
	return strings.Join(strings.Fields(code), "")
}

func normalizeRecoveryCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func generateOpaqueToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashOpaque(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
