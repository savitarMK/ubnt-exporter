package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"ubnt-exporter/internal/config"
)

// healthEndpoint returns an HTTP client and URL reaching the local daemon's /healthz endpoint.
// TCP listeners are preferred; when the daemon only listens on Unix domain sockets, the client
// dials the first socket directly.
func healthEndpoint(sysConf *config.SystemConfig) (*http.Client, string) {
	client := &http.Client{Timeout: 3 * time.Second}
	if sysConf == nil {
		sysConf = config.DefaultSystemConfig()
	}
	addrs, err := config.ParseListenAddresses(sysConf.Server.Listen, sysConf.Server.Port)
	if err == nil && len(addrs) > 0 {
		hasTCP := false
		for _, a := range addrs {
			if !strings.HasPrefix(a, "unix://") {
				hasTCP = true
				break
			}
		}
		if !hasTCP {
			sock := strings.TrimPrefix(addrs[0], "unix://")
			client.Transport = &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", sock)
				},
			}
			return client, "http://unix/healthz"
		}
	}
	return client, resolveLocalHTTPBase(sysConf) + "/healthz"
}

// CmdHealth checks the running daemon's /healthz endpoint and returns 0 when healthy.
// It is used as the container HEALTHCHECK and resolves the address from system.yaml,
// so it keeps working when server.listen or server.port change.
func CmdHealth(sysConf *config.SystemConfig) int {
	client, url := healthEndpoint(sysConf)
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unhealthy: %v\n", err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "unhealthy: %s returned HTTP %d\n", url, resp.StatusCode)
		return 1
	}
	fmt.Println("ok")
	return 0
}
