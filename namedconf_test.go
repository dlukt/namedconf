package namedconf_test

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/dlukt/namedconf"
)

func TestNamedConf(t *testing.T) {
	file, err := os.Open("/home/darko/named.conf")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	parser := namedconf.NewParser(file)
	config, err := parser.Parse()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Parsed configuration:\n%s", config.String())

	// Print TLS configurations
	for _, tls := range config.TLS {
		fmt.Printf("TLS Config: %s\n", tls.Name)
		fmt.Printf("  Cert: %s\n", tls.CertFile)
		fmt.Printf("  Key: %s\n", tls.KeyFile)
	}

	// Print zones with their advanced options
	for _, zone := range config.Zones {
		fmt.Printf("Zone: %s (%s)\n", zone.Name, zone.Type)
		if zone.DNSSECPolicy != "" {
			fmt.Printf("  DNSSEC Policy: %s\n", zone.DNSSECPolicy)
		}
		if zone.InlineSigning != nil && *zone.InlineSigning {
			fmt.Printf("  Inline Signing: enabled\n")
		}
		if len(zone.AllowTransfer) > 0 {
			fmt.Printf("  Allow Transfer: %v\n", zone.AllowTransfer)
		}
	}
}
