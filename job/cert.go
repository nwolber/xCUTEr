// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"strings"

	errs "github.com/pkg/errors"
)

const (
	pemBlockPrivateKey = "PRIVATE KEY"
)

func readPrivateKeyFile(keyFile string, password []byte) (crypto.Signer, []byte, error) {
	b, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, []byte{}, errs.Wrap(err, "failed to read file")
	}

	block, err := parsePEMBlock(b, pemBlockPrivateKey, password)
	if err != nil {
		return nil, []byte{}, errs.Wrapf(err, "failed to parse PEM block %s", pemBlockPrivateKey)
	}

	key, err := parsePrivateKey(block.Bytes)
	if err != nil {
		return nil, []byte{}, errs.Wrap(err, "failed to parse private key")
	}

	switch k := key.(type) {
	case crypto.Signer:
		return k, block.Bytes, nil
	default:
		return nil, []byte{}, errs.New("private key is no valid crypto.Signer")
	}
}

func parsePEMBlock(pemBlock []byte, typ string, password []byte) (*pem.Block, error) {
	var keyDERBlock *pem.Block
	for {
		keyDERBlock, pemBlock = pem.Decode(pemBlock)
		if keyDERBlock == nil {
			return nil, errs.New("can't read PEM block")
		}
		if x509.IsEncryptedPEMBlock(keyDERBlock) {
			out, err := x509.DecryptPEMBlock(keyDERBlock, password)
			if err != nil {
				return nil, errs.Wrap(err, "failed to decrypt PEM block")
			}
			keyDERBlock.Bytes = out
			break
		}
		if keyDERBlock.Type == typ || strings.HasSuffix(keyDERBlock.Type, " "+typ) {
			break
		}
	}

	return keyDERBlock, nil
}

func parsePrivateKey(der []byte) (crypto.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey:
			return key, nil
		default:
			return nil, errs.New("found unknown private key type in PKCS#8 wrapping")
		}
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}

	return nil, errs.New("failed to parse private key")
}
