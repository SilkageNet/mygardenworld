package babigame

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Official RN RequestSecurity constants (moac_rn 3.3.2.54).
const (
	requestSecurityAESIV      = "rI9SFNo4PYP8^6MQ" // exactly 16 bytes
	requestSecurityIVVersion  = "1"
	accountLoginUsernameV5Path = "/account/login/username/v5"
)

// deriveRequestSecurityAESKey matches RequestSecurity.deriveAesKey:
// SHA256( MD5(deviceId + nonce + timestamp).hex decoded as bytes ).
func deriveRequestSecurityAESKey(deviceID, nonce string, timestampMs int64) []byte {
	md5Hex := MD5Hex(deviceID + nonce + strconv.FormatInt(timestampMs, 10))
	md5Bytes, err := hex.DecodeString(md5Hex)
	if err != nil {
		// MD5Hex always returns valid hex; keep panic-free for safety.
		sum := md5.Sum([]byte(deviceID + nonce + strconv.FormatInt(timestampMs, 10)))
		md5Bytes = sum[:]
	}
	sum := sha256.Sum256(md5Bytes)
	return sum[:]
}

// buildRequestSecurityCanonical matches RequestSecurity.buildCanonical:
// METHOD + "\n" + PATH + "\n" + sorted key=value pairs (skip objects / null / fullSignature).
func buildRequestSecurityCanonical(method, path string, inner map[string]any) string {
	keys := make([]string, 0, len(inner))
	for k, v := range inner {
		if k == "fullSignature" || v == nil {
			continue
		}
		switch v.(type) {
		case map[string]any, []any:
			continue
		default:
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+fmt.Sprint(inner[k]))
	}
	return method + "\n" + path + "\n" + strings.Join(parts, "&")
}

func signRequestSecurityCanonical(canonical, deviceID string) string {
	mac := hmac.New(sha256.New, []byte(deviceID))
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+pad)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("requestsecurity: invalid pkcs7 length")
	}
	pad := int(data[len(data)-1])
	if pad <= 0 || pad > blockSize || pad > len(data) {
		return nil, fmt.Errorf("requestsecurity: invalid pkcs7 pad")
	}
	for i := len(data) - pad; i < len(data); i++ {
		if data[i] != byte(pad) {
			return nil, fmt.Errorf("requestsecurity: bad pkcs7 bytes")
		}
	}
	return data[:len(data)-pad], nil
}

func aes256CBCEncrypt(plain, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("requestsecurity: iv must be %d bytes", aes.BlockSize)
	}
	padded := pkcs7Pad(plain, aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return out, nil
}

func aes256CBCDecrypt(cipherText, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("requestsecurity: iv must be %d bytes", aes.BlockSize)
	}
	if len(cipherText) == 0 || len(cipherText)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("requestsecurity: invalid cipher length")
	}
	out := make([]byte, len(cipherText))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, cipherText)
	return pkcs7Unpad(out, aes.BlockSize)
}

func encryptRequestSecurityInner(plainJSON string, key []byte, iv string) (string, error) {
	ct, err := aes256CBCEncrypt([]byte(plainJSON), key, []byte(iv))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}

func decryptRequestSecurityInner(b64 string, key []byte, iv string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	return aes256CBCDecrypt(raw, key, []byte(iv))
}

// buildEncryptedOuterBody matches RequestSecurity.buildEncryptedOuterBody.
// Returns outer {timestamp, data} ready for the V5 HTTP body.
func buildEncryptedOuterBody(inner map[string]any, deviceID, nonce, method, path string, timestampMs int64) (map[string]any, error) {
	if timestampMs <= 0 {
		timestampMs = time.Now().UnixMilli()
	}
	body := make(map[string]any, len(inner)+1)
	for k, v := range inner {
		body[k] = v
	}
	canonical := buildRequestSecurityCanonical(method, path, body)
	body["fullSignature"] = signRequestSecurityCanonical(canonical, deviceID)
	plain, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	key := deriveRequestSecurityAESKey(deviceID, nonce, timestampMs)
	data, err := encryptRequestSecurityInner(string(plain), key, requestSecurityAESIV)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"timestamp": timestampMs,
		"data":      data,
	}, nil
}

func decryptV5ResponseData(encryptData, deviceID, nonce string, timestampMs int64) (map[string]any, error) {
	key := deriveRequestSecurityAESKey(deviceID, nonce, timestampMs)
	plain, err := decryptRequestSecurityInner(encryptData, key, requestSecurityAESIV)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, fmt.Errorf("requestsecurity: decrypt json: %w", err)
	}
	return out, nil
}

func generateV5Nonce() string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("v5-%d-%s", time.Now().UnixMilli(), hex.EncodeToString(b[:]))
}

// v5LoginInner builds the SecureRequestBuilder fingerprint + login overlays
// for POST /account/login/username/v5.
func (c *HTTPClient) v5LoginInner(username, passwordB64 string) map[string]any {
	ramGB := ""
	if mb, err := strconv.Atoi(c.Cfg.RAMMB); err == nil && mb > 0 {
		ramGB = strconv.Itoa((mb + 1023) / 1024)
	}
	resolution := ""
	if c.Cfg.ScreenWidthPx != "" && c.Cfg.ScreenHeightPx != "" {
		resolution = c.Cfg.ScreenWidthPx + "x" + c.Cfg.ScreenHeightPx
	}
	inst := "inst-" + c.DeviceID
	if len(c.DeviceID) >= 12 {
		inst = "inst-" + c.DeviceID[:12]
	}
	// Fingerprint shape matches SecureRequestBuilder.buildFullFingerprintInner
	// (moac_rn 3.3.2.54). On iOS, android_id is filled with the deviceId (not
	// the literal "empty"); language prefers rnLang-style "zh-CN".
	lang := c.Cfg.SysLanguage
	if lang == "" || lang == "zh-Hans-CN" {
		lang = "zh-CN"
	}
	inner := map[string]any{
		"appId":                    c.Cfg.AppID,
		"platform":                 "iOS",
		"version":                  "v" + c.Cfg.AppVersion,
		"packageName":              c.Cfg.PackageName,
		"clid":                     "1",
		"deviceId":                 c.DeviceID,
		"requestSecurityIvVersion": requestSecurityIVVersion,
		"gpu":                      "",
		"os_version":               c.Cfg.OSVersion,
		"language":                 lang,
		"carrier":                  "",
		"sdk_version":              c.Cfg.OSVersion,
		"timezone":                 "Asia/Shanghai",
		"cpu":                      c.Cfg.CPUType,
		"model":                    c.Cfg.DeviceModel,
		"network_type":             c.Cfg.NetworkType,
		"resolution":               resolution,
		"ram_gb":                   ramGB,
		"brand":                    c.Cfg.DeviceBrand,
		"manufacturer":             c.Cfg.DeviceBrand,
		"device_product":           "",
		"app_instance_id":          inst,
		"keychain_id":              c.DeviceID,
		"idfv":                     c.DeviceID,
		"darwin_version":           "",
		// Official iOS SecureRequestBuilder sets android_id = appInfo.deviceId.
		"android_id":        c.DeviceID,
		"build_fingerprint": "",
		"oaid":              "",
		// Business overlays (account login).
		"name":                   username,
		"value":                  passwordB64,
		"lang":                   "zh",
		"showVerEighteenAgeTips": false,
		"storeCountryCode":       "CN",
		"sysLanguage":            c.Cfg.SysLanguage,
	}
	return inner
}
