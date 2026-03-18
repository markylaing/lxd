//go:build linux && cgo && !agent

package db_test

import (
	"context"
	"encoding/pem"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/canonical/lxd/lxd/certificate"
	"github.com/canonical/lxd/lxd/db"
	"github.com/canonical/lxd/lxd/db/cluster"
	"github.com/canonical/lxd/shared"
	"github.com/canonical/lxd/shared/api"
)

func TestGetCertificate(t *testing.T) {
	tx, cleanup := db.NewTestClusterTx(t)
	defer cleanup()

	testCertPair := shared.TestingKeyPair()
	testCert, err := testCertPair.PublicKeyX509()
	require.NoError(t, err)
	testCertString := string(pem.EncodeToMemory(&pem.Block{Bytes: testCert.Raw, Type: "CERTIFICATE"}))
	fingerprint := shared.CertFingerprint(testCert)

	ctx := context.Background()
	_, err = cluster.CreateCertificateLegacy(ctx, tx.Tx(), cluster.CertificateLegacy{
		Name:        "test-cert",
		Fingerprint: fingerprint,
		Certificate: testCertString,
		Type:        certificate.TypeClient,
	})
	require.NoError(t, err)

	cert, err := cluster.GetCertificateLegacy(ctx, tx.Tx(), "incorrect")
	require.Error(t, err)
	require.True(t, api.StatusErrorCheck(err, http.StatusNotFound))
	require.Nil(t, cert)

	cert, err = cluster.GetCertificateLegacy(ctx, tx.Tx(), fingerprint)
	require.NoError(t, err)
	assert.Equal(t, fingerprint, cert.Fingerprint)
	assert.Equal(t, "test-cert", cert.Name)
	assert.Equal(t, testCertString, cert.Certificate)
	assert.Equal(t, certificate.TypeClient, cert.Type)
}
