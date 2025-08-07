package tlsinfo

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"net"
	"time"

	"github.com/gustycube/spyder-probe/internal/emit"
)

func FetchCert(host string) (*emit.NodeCert, error) {
	d := &tls.Dialer{Config: &tls.Config{ServerName: host}}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, "443"))
	if err != nil { return nil, err }
	defer conn.Close()
	cs := conn.(*tls.Conn).ConnectionState()
	if len(cs.PeerCertificates) == 0 { return nil, nil }
	leaf := cs.PeerCertificates[0]
	spki := sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	return &emit.NodeCert{
		SPKI:      base64.StdEncoding.EncodeToString(spki[:]),
		SubjectCN: leaf.Subject.CommonName,
		IssuerCN:  leaf.Issuer.CommonName,
		NotBefore: leaf.NotBefore,
		NotAfter:  leaf.NotAfter,
	}, nil
}
