package terminal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const APIBase = "https://api.romaine.life/at"

// IdentityClaims holds JWT payload claims for an identity.
type IdentityClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// LoadIdentityClaims reads the identity name and claims from configDir.
func LoadIdentityClaims(configDir string) (string, IdentityClaims, error) {
	identityName, err := ReadTrimmedFile(filepath.Join(configDir, ".identity"))
	if err != nil {
		return "", IdentityClaims{}, fmt.Errorf("no identity loaded — use load first")
	}

	data, err := os.ReadFile(filepath.Join(configDir, "identities.json"))
	if err != nil {
		return "", IdentityClaims{}, fmt.Errorf("identities.json not found")
	}
	var all map[string]IdentityClaims
	if err := json.Unmarshal(data, &all); err != nil {
		return "", IdentityClaims{}, fmt.Errorf("invalid identities.json: %w", err)
	}
	c, ok := all[identityName]
	if !ok {
		return "", IdentityClaims{}, fmt.Errorf("unknown identity: %s", identityName)
	}
	return identityName, c, nil
}

// MintJWT creates a short-lived HS256 JWT (5 minute expiry).
func MintJWT(secret string, claims IdentityClaims) string {
	header := Base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

	now := time.Now().Unix()
	payload := fmt.Sprintf(`{"sub":"%s","email":"%s","name":"%s","role":"%s","iat":%d,"exp":%d}`,
		claims.Sub, claims.Email, claims.Name, claims.Role, now, now+300)
	payloadEnc := Base64URLEncode([]byte(payload))

	msg := header + "." + payloadEnc
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	sig := Base64URLEncode(mac.Sum(nil))

	return msg + "." + sig
}

// FetchBookmarks GETs bookmarks from the API and returns the raw response.
func FetchBookmarks(token string) ([]interface{}, string, error) {
	req, err := http.NewRequest("GET", APIBase+"/api/bookmarks", nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	var result struct {
		Bookmarks []interface{} `json:"bookmarks"`
		UpdatedAt *string       `json:"updatedAt"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", err
	}

	updatedAt := ""
	if result.UpdatedAt != nil {
		updatedAt = *result.UpdatedAt
	}
	return result.Bookmarks, updatedAt, nil
}

// StripMetadata removes keys starting with "_" from bookmark objects recursively.
func StripMetadata(items []interface{}) []interface{} {
	var out []interface{}
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			out = append(out, item)
			continue
		}
		clean := make(map[string]interface{})
		for k, v := range m {
			if strings.HasPrefix(k, "_") {
				continue
			}
			if k == "children" {
				if children, ok := v.([]interface{}); ok {
					clean[k] = StripMetadata(children)
					continue
				}
			}
			clean[k] = v
		}
		out = append(out, clean)
	}
	return out
}

// FetchMenu GETs the full menu tree from the API.
func FetchMenu(token string) ([]interface{}, string, error) {
	req, err := http.NewRequest("GET", APIBase+"/api/menu", nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	var result struct {
		Menu      []interface{} `json:"menu"`
		UpdatedAt *string       `json:"updatedAt"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, "", err
	}

	updatedAt := ""
	if result.UpdatedAt != nil {
		updatedAt = *result.UpdatedAt
	}
	return result.Menu, updatedAt, nil
}

// SyncMenu fetches the full menu from the API and writes the cache file as YAML.
// Returns the number of top-level items synced, or an error.
func SyncMenu(configDir, secret string) (int, error) {
	_, claims, err := LoadIdentityClaims(configDir)
	if err != nil {
		return 0, err
	}

	token := MintJWT(secret, claims)
	menu, _, err := FetchMenu(token)
	if err != nil {
		return 0, fmt.Errorf("API error: %w", err)
	}

	data, err := MenuToYAML(menu)
	if err != nil {
		return 0, err
	}

	cacheFile := filepath.Join(configDir, "menu-cache.yaml")
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return 0, fmt.Errorf("failed to write cache: %w", err)
	}

	return len(menu), nil
}

// MenuToYAML converts the API menu response (JSON objects) to YAML format
// compatible with fzt's LoadYAML.
func MenuToYAML(items []interface{}) ([]byte, error) {
	return marshalYAMLItems(items, 0), nil
}

func marshalYAMLItems(items []interface{}, indent int) []byte {
	var buf []byte
	prefix := strings.Repeat("  ", indent)
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		desc, _ := m["description"].(string)
		url, _ := m["url"].(string)
		children, hasChildren := m["children"].([]interface{})

		buf = append(buf, []byte(prefix+"- name: \""+name+"\"\n")...)
		if desc != "" {
			buf = append(buf, []byte(prefix+"  description: \""+desc+"\"\n")...)
		}
		if url != "" {
			buf = append(buf, []byte(prefix+"  url: \""+url+"\"\n")...)
		}
		if hasChildren && len(children) > 0 {
			buf = append(buf, []byte(prefix+"  children:\n")...)
			buf = append(buf, marshalYAMLItems(children, indent+2)...)
		}
	}
	return buf
}

// Base64URLEncode encodes data as base64url without padding.
func Base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

// ReadTrimmedFile reads a file and returns its trimmed content.
func ReadTrimmedFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
