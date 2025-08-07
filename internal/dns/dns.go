package dns

import (
	"context"
	"net"
	"strings"
)

func ResolveAll(ctx context.Context, host string) (ips []string, nsHosts []string, cname string, mxHosts []string, txts []string) {
	ips, nsHosts, mxHosts, txts = []string{}, []string{}, []string{}, []string{}
	if iplist, err := net.DefaultResolver.LookupIP(ctx, "ip", host); err == nil {
		for _, ip := range iplist { ips = append(ips, ip.String()) }
	}
	if ns, err := net.DefaultResolver.LookupNS(ctx, host); err == nil {
		for _, n := range ns { nsHosts = append(nsHosts, strings.TrimSuffix(n.Host, ".")) }
	}
	if c, err := net.DefaultResolver.LookupCNAME(ctx, host); err == nil {
		cname = strings.TrimSuffix(c, ".")
	}
	if mxs, err := net.DefaultResolver.LookupMX(ctx, host); err == nil {
		for _, m := range mxs { mxHosts = append(mxHosts, strings.TrimSuffix(m.Host, ".")) }
	}
	if t, err := net.DefaultResolver.LookupTXT(ctx, host); err == nil {
		txts = append(txts, t...)
	}
	return
}
