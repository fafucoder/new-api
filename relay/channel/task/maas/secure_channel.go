package maas

// Jeddak Secure Channel implementation for 移动云 MaaS gateway.
//
// Protocol:
//  1. POST /v1/security/token  →  JWT whose payload contains a "jwk" field (RSA public key)
//  2. Generate random AES-256 key; encrypt plaintext with AES-GCM
//  3. Encrypt AES key with server RSA public key (OAEP-SHA256)
//  4. Serialize as {"key":b64,"nonce":b64,"mac":b64,"ciphertext":b64}
//  5. Send with X-AICC-Encryption-Enable: true header
//  6. Decrypt response with the same AES key (response has no "key" field)

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
)

const securityTokenPath = "/v1/security/token"
const aiccSDKVersion = "0.1.0"

// envelopeMsg is the wire format for both requests and responses.
type envelopeMsg struct {
	Key        string `json:"key,omitempty"`
	Nonce      string `json:"nonce"`
	Mac        string `json:"mac"`
	Ciphertext string `json:"ciphertext"`
}

// secureSession holds a cached server public key per base-URL.
type secureSession struct {
	pubKey    *rsa.PublicKey
	expiresAt time.Time
}

var (
	sessionCache   = map[string]*secureSession{}
	sessionCacheMu sync.RWMutex
)

// getServerPublicKey fetches (or returns cached) the server RSA public key.
// baseURL is the root URL without /api/v3, e.g. https://zhenze-huhehaote.cmecloud.cn/api/v3
func getServerPublicKey(baseURL, apiKey string) (*rsa.PublicKey, error) {
	// derive root URL: strip /api/v3 suffix for the security token endpoint
	rootURL := strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/api/v3")

	sessionCacheMu.RLock()
	if s, ok := sessionCache[rootURL]; ok && time.Now().Before(s.expiresAt) {
		sessionCacheMu.RUnlock()
		return s.pubKey, nil
	}
	sessionCacheMu.RUnlock()

	url := rootURL + securityTokenPath
	body, _ := common.Marshal(map[string]string{"Nonce": generateNonce()})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := service.GetHttpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("security token request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	pubKey, exp, err := extractPublicKeyFromTokenResponse(respBody)
	if err != nil {
		return nil, fmt.Errorf("extract public key failed: %w, body: %s", err, respBody)
	}

	sessionCacheMu.Lock()
	sessionCache[rootURL] = &secureSession{pubKey: pubKey, expiresAt: exp}
	sessionCacheMu.Unlock()
	return pubKey, nil
}

// extractPublicKeyFromTokenResponse parses the /v1/security/token response.
// It looks for pub_key_info (PEM) first, then falls back to JWT jwk payload.
func extractPublicKeyFromTokenResponse(body []byte) (*rsa.PublicKey, time.Time, error) {
	var raw map[string]interface{}
	if err := common.Unmarshal(body, &raw); err != nil {
		return nil, time.Time{}, err
	}

	// Try to find pub_key_info (PEM) anywhere in the nested structure
	if pem := findString(raw, "pub_key_info"); pem != "" {
		key, err := parsePEMPublicKey(pem)
		if err == nil {
			return key, time.Now().Add(30 * time.Minute), nil
		}
	}

	// Fall back to JWT token → jwk
	if token := findString(raw, "token"); token != "" {
		key, exp, err := extractKeyFromJWT(token)
		if err == nil {
			return key, exp, nil
		}
	}

	return nil, time.Time{}, fmt.Errorf("no public key found in response")
}

func findString(v interface{}, key string) string {
	switch m := v.(type) {
	case map[string]interface{}:
		for k, val := range m {
			if k == key {
				if s, ok := val.(string); ok {
					return s
				}
			}
			if s := findString(val, key); s != "" {
				return s
			}
		}
	case []interface{}:
		for _, item := range m {
			if s := findString(item, key); s != "" {
				return s
			}
		}
	}
	return ""
}

func parsePEMPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not RSA key")
	}
	return rsaKey, nil
}

func extractKeyFromJWT(token string) (*rsa.PublicKey, time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, time.Time{}, fmt.Errorf("invalid JWT")
	}
	padded := parts[1]
	for len(padded)%4 != 0 {
		padded += "="
	}
	payloadBytes, err := base64.URLEncoding.DecodeString(padded)
	if err != nil {
		return nil, time.Time{}, err
	}

	var payload struct {
		Exp int64 `json:"exp"`
		JWK struct {
			N   string `json:"n"`
			E   string `json:"e"`
			Kty string `json:"kty"`
		} `json:"jwk"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, time.Time{}, err
	}
	if payload.JWK.Kty != "RSA" || payload.JWK.N == "" {
		return nil, time.Time{}, fmt.Errorf("no RSA jwk in JWT")
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(payload.JWK.N)
	if err != nil {
		return nil, time.Time{}, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(payload.JWK.E)
	if err != nil {
		return nil, time.Time{}, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)
	pubKey := &rsa.PublicKey{N: n, E: int(e.Int64())}

	exp := time.Now().Add(30 * time.Minute)
	if payload.Exp > 0 {
		exp = time.Unix(payload.Exp, 0)
	}
	return pubKey, exp, nil
}

// encryptBody encrypts plaintext using envelope encryption and returns
// the encrypted body bytes and the AES key (for response decryption).
func encryptBody(plaintext []byte, pubKey *rsa.PublicKey) ([]byte, []byte, error) {
	// Generate random AES-256 key
	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		return nil, nil, err
	}

	// AES-GCM encrypt
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	// GCM Seal appends ciphertext+tag; we need to split them
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	ciphertext := sealed[:len(sealed)-16]
	mac := sealed[len(sealed)-16:]

	// RSA-OAEP-SHA256 encrypt AES key
	encryptedKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, aesKey, nil)
	if err != nil {
		return nil, nil, err
	}

	msg := envelopeMsg{
		Key:        base64.StdEncoding.EncodeToString(encryptedKey),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Mac:        base64.StdEncoding.EncodeToString(mac),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
	encoded, err := json.Marshal(msg)
	return encoded, aesKey, err
}

// decryptResponse decrypts an envelope-encrypted response body.
func decryptResponse(body []byte, aesKey []byte) ([]byte, error) {
	var msg envelopeMsg
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}

	nonce, err := base64.StdEncoding.DecodeString(msg.Nonce)
	if err != nil {
		return nil, err
	}
	mac, err := base64.StdEncoding.DecodeString(msg.Mac)
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(msg.Ciphertext)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Reconstruct ciphertext+tag for GCM Open
	combined := append(ciphertext, mac...)
	plaintext, err := gcm.Open(nil, nonce, combined, nil)
	if err != nil {
		return nil, fmt.Errorf("AES-GCM decrypt failed: %w", err)
	}
	return plaintext, nil
}

func generateNonce() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
