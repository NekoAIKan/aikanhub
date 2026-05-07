package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

func normalizeRSAPrivateKey(raw string) (string, error) {
	return normalizePEMKey(raw, "PRIVATE KEY", "RSA PRIVATE KEY")
}

func normalizeRSAPublicKey(raw string) (string, error) {
	return normalizePEMKey(raw, "PUBLIC KEY", "RSA PUBLIC KEY")
}

func normalizePEMKey(raw string, pkcs8Type string, pkcs1Type string) (string, error) {
	normalized := strings.TrimSpace(strings.ReplaceAll(raw, `\n`, "\n"))
	if normalized == "" {
		return "", fmt.Errorf("%s is empty", strings.ToLower(pkcs8Type))
	}
	if strings.Contains(normalized, "BEGIN ") {
		block, _ := pem.Decode([]byte(normalized))
		if block == nil {
			return "", fmt.Errorf("invalid PEM encoded %s", strings.ToLower(pkcs8Type))
		}
		return string(pem.EncodeToMemory(block)), nil
	}

	der, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(normalized, "\n", ""))
	if err != nil {
		return "", fmt.Errorf("invalid base64 encoded %s: %w", strings.ToLower(pkcs8Type), err)
	}

	pemType := pkcs8Type
	if pkcs8Type == "PRIVATE KEY" {
		if _, err := x509.ParsePKCS8PrivateKey(der); err != nil {
			if _, err := x509.ParsePKCS1PrivateKey(der); err == nil {
				pemType = pkcs1Type
			} else {
				return "", fmt.Errorf("invalid RSA private key")
			}
		}
	} else {
		if _, err := x509.ParsePKIXPublicKey(der); err != nil {
			if _, err := x509.ParsePKCS1PublicKey(der); err == nil {
				pemType = pkcs1Type
			} else {
				return "", fmt.Errorf("invalid RSA public key")
			}
		}
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: pemType, Bytes: der})), nil
}

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	keyPEM, err := normalizeRSAPrivateKey(raw)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, fmt.Errorf("invalid RSA private key PEM")
	}
	switch block.Type {
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#8 private key: %w", err)
		}
		parsed, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
		return parsed, nil
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#1 private key: %w", err)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported private key type: %s", block.Type)
	}
}

func parseRSAPublicKey(raw string) (*rsa.PublicKey, error) {
	keyPEM, err := normalizeRSAPublicKey(raw)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, fmt.Errorf("invalid RSA public key PEM")
	}
	switch block.Type {
	case "PUBLIC KEY":
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKIX public key: %w", err)
		}
		parsed, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("public key is not RSA")
		}
		return parsed, nil
	case "RSA PUBLIC KEY":
		key, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#1 public key: %w", err)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported public key type: %s", block.Type)
	}
}

func alipaySigningContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}

func signAlipayParams(params map[string]string, privateKeyRaw string) (string, error) {
	privateKey, err := parseRSAPrivateKey(privateKeyRaw)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(alipaySigningContent(params)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign Alipay params: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func verifyAlipayParams(values url.Values, publicKeyRaw string) error {
	signatureRaw := values.Get("sign")
	if signatureRaw == "" {
		return fmt.Errorf("missing Alipay sign")
	}
	params := map[string]string{}
	for key, vals := range values {
		if len(vals) > 0 {
			params[key] = vals[0]
		}
	}
	publicKey, err := parseRSAPublicKey(publicKeyRaw)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(signatureRaw)
	if err != nil {
		return fmt.Errorf("decode Alipay sign: %w", err)
	}
	digest := sha256.Sum256([]byte(alipaySigningContent(params)))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return fmt.Errorf("verify Alipay sign: %w", err)
	}
	return nil
}
