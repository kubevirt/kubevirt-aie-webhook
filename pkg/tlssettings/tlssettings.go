package tlssettings

import (
	"crypto/tls"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	cliflag "k8s.io/component-base/cli/flag"
)

func AppendCipherSuites(logger logr.Logger, tlsOpts []func(*tls.Config), cipherSuitesFlag string) []func(*tls.Config) {
	if cipherSuitesFlag == "" {
		return tlsOpts
	}
	cipherSuites := strings.Split(cipherSuitesFlag, ",")
	cipherSuiteIDs, err := cliflag.TLSCipherSuites(cipherSuites)
	if err != nil {
		logger.Error(err, "failed to parse TLS cipher suites")
		os.Exit(1)
	}

	setCipherSuites := func(c *tls.Config) {
		logger.Info("setting tls cipher suites to " + strings.Join(cipherSuites, ", "))
		c.CipherSuites = cipherSuiteIDs
	}

	return append(tlsOpts, setCipherSuites)
}

func AppendMinTLSVersion(logger logr.Logger, tlsOpts []func(*tls.Config), minTLSVersion string) []func(*tls.Config) {
	if minTLSVersion == "" {
		return tlsOpts
	}
	minTLSVersionID, err := cliflag.TLSVersion(minTLSVersion)
	if err != nil {
		logger.Error(err, "failed to parse TLS min version")
		os.Exit(1)
	}

	setMinTLSVersion := func(c *tls.Config) {
		logger.Info("setting tls min version to " + minTLSVersion)
		c.MinVersion = minTLSVersionID
	}

	return append(tlsOpts, setMinTLSVersion)
}

func AppendCurveIds(logger logr.Logger, tlsOpts []func(*tls.Config), curveIdsString string) ([]func(*tls.Config), error) {
	if curveIdsString == "" {
		return tlsOpts, nil
	}

	curveStrings := strings.Split(curveIdsString, ",")
	curveIds := make([]tls.CurveID, 0, len(curveStrings))
	for _, idStr := range curveStrings {
		parseUint, err := strconv.ParseUint(idStr, 10, 16)
		if err != nil {
			logger.Error(err, "failed to parse curveId", "curveID", idStr)
			continue
		}

		curveId := tls.CurveID(parseUint)
		if _, ok := validCurveIDs[curveId]; !ok {
			logger.Error(err, "unsupported curveId", "curveID", idStr)
			continue
		}

		curveIds = append(curveIds, curveId)
	}

	if len(curveIds) == 0 {
		logger.Error(nil, "no valid curveIds found", "curveIdConfiguration", curveIdsString)
		return nil, errors.New("no valid curveIds found")
	}

	setCurveIDs := func(c *tls.Config) {
		logger.Info("setting tls curveIds", "curveIds", curveIdsString)
		c.CurvePreferences = curveIds
	}

	return append(tlsOpts, setCurveIDs), nil
}

var validCurveIDs = map[tls.CurveID]struct{}{
	tls.CurveP256:          {},
	tls.CurveP384:          {},
	tls.CurveP521:          {},
	tls.X25519:             {},
	tls.X25519MLKEM768:     {},
	tls.SecP256r1MLKEM768:  {},
	tls.SecP384r1MLKEM1024: {},
}
