/*
 * Adsisto
 * Copyright (c) 2019 Andrew Ying
 *
 * This program is free software: you can redistribute it and/or modify it under
 * the terms of version 3 of the GNU General Public License as published by the
 * Free Software Foundation. In addition, this program is also subject to certain
 * additional terms available at <SUPPLEMENT.md>.
 *
 * This program is distributed in the hope that it will be useful, but WITHOUT ANY
 * WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
 * A PARTICULAR PURPOSE.  See the GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License along with
 * this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package auth

import (
	"errors"
	"github.com/SermoDigital/jose/crypto"
	"github.com/SermoDigital/jose/jws"
	"github.com/SermoDigital/jose/jwt"
	"gopkg.in/go-playground/validator.v9"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type JWTMiddleware struct {
	// Signing algorithm
	SigningAlgorithm string
	// Path to server public key
	PubKeyPath string
	// Path to server private key
	PrivKeyPath string
	// Server public key
	PubKey interface{}
	// Server private key, should be of type *rsa.PrivKey or *ecdsa.PrivKey
	PrivKey interface{}
	// Name of the authorised key interface
	Interface string
	// Interface configuration
	InterfaceConfig map[string]string
	// An AuthorisedKeyInterface instance
	AuthorisedKeys KeysStoreInterface
	CookieName     string
	AuthnTimeout   time.Duration
	SessionTimeout time.Duration
	Leeway         time.Duration
	Validator      *validator.Validate
	Unauthorised   func(int, http.ResponseWriter)
}

type KeysStoreInterface interface {
	New(map[string]string)
	Get(...interface{}) (KeyInstance, error)
	GetAll() (interface{}, error)
	Insert(...string) error
	Update(...string) error
	Delete(...interface{}) error
}

type KeyInstance struct {
	Key         string
	AccessLevel int
}

var (
	// Mapping between name and instances of KeysStoreInterface
	keysInterfaces = map[string]KeysStoreInterface{
		"mysql": &MysqlKeyStore{},
	}

	ErrMethodNotImplemented = errors.New("method not implemented by key store")
	ErrKeyNotFound          = errors.New("authorised key not found")
	ErrInvalidAlg           = errors.New("signing algorithm is invalid")
	ErrHMACAlg              = errors.New("HMAC algorithms are not accepted")
	ErrMissingPubKey        = errors.New("public key is required")
	ErrMissingPrivKey       = errors.New("private key is required")
	ErrInvalidExpDuration   = errors.New("expiration is longer than the permitted duration")
	ErrInvalidToken         = errors.New("invalid JWT")
)

// MiddlewareInit is responsible for the setting up of the authentication
// middleware.
func (m *JWTMiddleware) MiddlewareInit() error {
	switch strings.ToUpper(m.SigningAlgorithm) {
	case "RS256":
	case "RS384":
	case "RS512":
	case "ES256":
	case "ES384":
	case "ES512":
		break
	case "HS256":
	case "HS384":
	case "HS512":
		return ErrHMACAlg
	default:
		return ErrInvalidAlg
	}

	if m.PubKeyPath != "" && m.PubKey == nil {
		keyData, err := ioutil.ReadFile(m.PubKeyPath)
		if err != nil {
			return ErrMissingPubKey
		}

		key, err := m.parsePublicKey(keyData)
		if err != nil {
			return ErrMissingPubKey
		}

		m.PubKey = key
	}

	if m.PubKey == nil {
		return ErrMissingPubKey
	}

	if m.PrivKeyPath != "" && m.PrivKey == nil {
		keyData, err := ioutil.ReadFile(m.PrivKeyPath)
		if err != nil {
			return ErrMissingPrivKey
		}

		key, err := m.parsePrivateKey(keyData)
		if err != nil {
			return ErrMissingPrivKey
		}

		m.PrivKey = key
	}

	if m.PrivKey == nil {
		return ErrMissingPrivKey
	}

	if m.AuthorisedKeys == nil {
		m.AuthorisedKeys = keysInterfaces[m.Interface]
		m.AuthorisedKeys.New(m.InterfaceConfig)
	}

	m.Validator = validator.New()
	if err := m.Validator.RegisterValidation(
		"uniqueIdentity",
		m.uniqueIdentityValidator,
	); err != nil {
		return err
	}
	if err := m.Validator.RegisterValidation(
		"existsIdentity",
		m.existsIdentityValidator,
	); err != nil {
		return err
	}
	if err := m.Validator.RegisterValidation("publicKey", PublicKeyValidator); err != nil {
		return err
	}
	return nil
}

// ValidateAuthnRequest validates authentication request for a validly signed JWT
func (m *JWTMiddleware) ValidateAuthnRequest(t string) (interface{}, error) {
	log.Printf(
		"[INFO] Validating authentication token \"%s\"\n",
		t,
	)

	token, err := jws.ParseJWT([]byte(t))
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims := token.Claims()
	issuer := claims.Get("iss")
	log.Printf("[INFO] Parsed authentication token from %s", issuer)

	validate := jws.NewValidator(
		jws.Claims{},
		m.Leeway,
		m.Leeway,
		func(claims jws.Claims) error {
			exp := time.Unix(claims.Get("exp").(int64), 0)
			iat := time.Unix(claims.Get("iat").(int64), 0)

			expectedExp := iat.Add(m.AuthnTimeout)
			if expectedExp.Before(exp) {
				return ErrInvalidExpDuration
			}

			return nil
		},
	)

	key, err := m.AuthorisedKeys.Get(issuer)
	if key == (KeyInstance{}) {
		return nil, nil
	}
	if err != nil {
		log.Print(err)
		return nil, err
	}

	err = token.Validate(
		key.Key,
		jws.GetSigningMethod(m.SigningAlgorithm),
		validate,
	)
	if err != nil {
		log.Print(err)
		return nil, nil
	}

	return key, nil
}

// GetSessionToken generate session token, in the form of a valid JWT signed
// using the server's private key.
func (m *JWTMiddleware) GetSessionToken(data interface{}) (string, error) {
	now := time.Now()

	claim := jws.Claims{}
	claim.SetIssuedAt(now)
	claim.SetNotBefore(now)
	claim.SetExpiration(now.Add(m.SessionTimeout))
	claim.Set("user", data)

	token := jws.NewJWT(claim, jws.GetSigningMethod(m.SigningAlgorithm))
	bytes, err := token.Serialize(m.PrivKey)
	if err != nil {
		return "", err
	}

	return string(bytes[:]), nil
}

// ValidateSessionToken validates the validity of the JWT token.
func (m *JWTMiddleware) ValidateSessionToken(t jwt.JWT) (bool, error) {
	validate := jws.NewValidator(
		jws.Claims{},
		m.Leeway,
		m.Leeway,
		func(claims jws.Claims) error {
			return nil
		},
	)

	err := t.Validate(
		m.PubKey,
		jws.GetSigningMethod(m.SigningAlgorithm),
		validate,
	)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (m *JWTMiddleware) parsePublicKey(k []byte) (interface{}, error) {
	switch strings.ToUpper(m.SigningAlgorithm) {
	case "RS256":
	case "RS384":
	case "RS512":
		return crypto.ParseRSAPublicKeyFromPEM(k)
	case "ES256":
	case "ES384":
	case "ES512":
		return crypto.ParseECPublicKeyFromPEM(k)
	}

	return nil, ErrInvalidAlg
}

func (m *JWTMiddleware) parsePrivateKey(k []byte) (interface{}, error) {
	switch strings.ToUpper(m.SigningAlgorithm) {
	case "RS256":
	case "RS384":
	case "RS512":
		return crypto.ParseRSAPrivateKeyFromPEM(k)
	case "ES256":
	case "ES384":
	case "ES512":
		return crypto.ParseECPrivateKeyFromPEM(k)
	}

	return nil, ErrInvalidAlg
}
