package namedconf

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// Config represents the entire named.conf configuration
type Config struct {
	Zones      []Zone             `json:"zones"`
	Options    Options            `json:"options"`
	ACLs       []ACL              `json:"acls"`
	Keys       []Key              `json:"keys"`
	TLS        []TLSConfig        `json:"tls"`
	Logging    *Logging           `json:"logging,omitempty"`
	Controls   *Controls          `json:"controls,omitempty"`
	Includes   []string           `json:"includes"`
	Masters    []Masters          `json:"masters"`
	Servers    []Server           `json:"servers"`
	Views      []View             `json:"views"`
	Statistics *Statistics        `json:"statistics,omitempty"`
	Unknown    []UnknownStatement `json:"unknown"`
}

// TLSConfig represents a TLS configuration block
type TLSConfig struct {
	Name     string `json:"name"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

// ListenOn represents a listen-on configuration
type ListenOn struct {
	Port      int      `json:"port"`
	TLS       string   `json:"tls,omitempty"`
	HTTP      string   `json:"http,omitempty"`
	Addresses []string `json:"addresses"`
}

// Zone represents a zone configuration
type Zone struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	File          string            `json:"file,omitempty"`
	Masters       []string          `json:"masters,omitempty"`
	Notify        *bool             `json:"notify,omitempty"`
	DNSSECPolicy  string            `json:"dnssec_policy,omitempty"`
	InlineSigning *bool             `json:"inline_signing,omitempty"`
	AllowTransfer []string          `json:"allow_transfer,omitempty"`
	Options       map[string]string `json:"options"`
	Comments      []string          `json:"comments,omitempty"`
}

// Options represents the global options block
type Options struct {
	Directory            string            `json:"directory,omitempty"`
	PidFile              string            `json:"pid_file,omitempty"`
	ListenOn             []ListenOn        `json:"listen_on,omitempty"`
	ListenOnV6           []ListenOn        `json:"listen_on_v6,omitempty"`
	QuerySource          string            `json:"query_source,omitempty"`
	Forwarders           []string          `json:"forwarders,omitempty"`
	Forward              string            `json:"forward,omitempty"`
	Recursion            *bool             `json:"recursion,omitempty"`
	AllowQuery           []string          `json:"allow_query,omitempty"`
	AllowQueryCache      []string          `json:"allow_query_cache,omitempty"`
	AllowTransfer        []string          `json:"allow_transfer,omitempty"`
	AllowRecursion       []string          `json:"allow_recursion,omitempty"`
	AllowNewZones        *bool             `json:"allow_new_zones,omitempty"`
	AlsoNotify           []string          `json:"also_notify,omitempty"`
	Version              string            `json:"version,omitempty"`
	Hostname             string            `json:"hostname,omitempty"`
	ServerID             string            `json:"server_id,omitempty"`
	NotifySource         string            `json:"notify_source,omitempty"`
	TransferSource       string            `json:"transfer_source,omitempty"`
	DNSSECValidation     string            `json:"dnssec_validation,omitempty"`
	DumpFile             string            `json:"dump_file,omitempty"`
	StatisticsFile       string            `json:"statistics_file,omitempty"`
	MemstatisticsFile    string            `json:"memstatistics_file,omitempty"`
	SecrootsFile         string            `json:"secroots_file,omitempty"`
	RecursingFile        string            `json:"recursing_file,omitempty"`
	ManagedKeysDirectory string            `json:"managed_keys_directory,omitempty"`
	GeoipDirectory       string            `json:"geoip_directory,omitempty"`
	SessionKeyfile       string            `json:"session_keyfile,omitempty"`
	Additional           map[string]string `json:"additional"`
}

// ACL represents an access control list
type ACL struct {
	Name    string   `json:"name"`
	Entries []string `json:"entries"`
}

// Key represents a TSIG key
type Key struct {
	Name      string `json:"name"`
	Algorithm string `json:"algorithm"`
	Secret    string `json:"secret"`
}

// Logging represents the logging configuration
type Logging struct {
	Channels   []LogChannel  `json:"channels"`
	Categories []LogCategory `json:"categories"`
}

// LogChannel represents a logging channel
type LogChannel struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	File          string            `json:"file,omitempty"`
	Severity      string            `json:"severity,omitempty"`
	PrintTime     *bool             `json:"print_time,omitempty"`
	PrintSeverity *bool             `json:"print_severity,omitempty"`
	PrintCategory *bool             `json:"print_category,omitempty"`
	Additional    map[string]string `json:"additional"`
}

// LogCategory represents a logging category
type LogCategory struct {
	Name     string   `json:"name"`
	Channels []string `json:"channels"`
}

// Controls represents the controls configuration
type Controls struct {
	Inet []ControlInet `json:"inet"`
	Unix []ControlUnix `json:"unix"`
}

// ControlInet represents inet controls
type ControlInet struct {
	Address string   `json:"address"`
	Port    int      `json:"port"`
	Allow   []string `json:"allow"`
	Keys    []string `json:"keys"`
}

// ControlUnix represents unix socket controls
type ControlUnix struct {
	Path  string `json:"path"`
	Perm  string `json:"perm,omitempty"`
	Owner string `json:"owner,omitempty"`
	Group string `json:"group,omitempty"`
}

// Masters represents a masters definition
type Masters struct {
	Name    string   `json:"name"`
	Masters []string `json:"masters"`
}

// Server represents a server statement
type Server struct {
	Address        string            `json:"address"`
	BogusAnswer    *bool             `json:"bogus_answer,omitempty"`
	Edns           *bool             `json:"edns,omitempty"`
	Keys           []string          `json:"keys,omitempty"`
	TransferFormat string            `json:"transfer_format,omitempty"`
	Additional     map[string]string `json:"additional"`
}

// View represents a view configuration
type View struct {
	Name    string            `json:"name"`
	Class   string            `json:"class,omitempty"`
	Zones   []Zone            `json:"zones"`
	Options map[string]string `json:"options"`
	Match   []string          `json:"match,omitempty"`
}

// Statistics represents statistics channels
type Statistics struct {
	Channels []StatChannel `json:"channels"`
}

// StatChannel represents a statistics channel
type StatChannel struct {
	Address string   `json:"address"`
	Port    int      `json:"port"`
	Allow   []string `json:"allow"`
}

// UnknownStatement represents unparsed statements
type UnknownStatement struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// Parser represents the named.conf parser
type Parser struct {
	scanner *bufio.Scanner
	line    int
	current string
	tokens  []string
	pos     int
}

// NewParser creates a new parser for the given reader
func NewParser(r io.Reader) *Parser {
	return &Parser{
		scanner: bufio.NewScanner(r),
		line:    0,
	}
}

// Parse parses the entire named.conf file
func (p *Parser) Parse() (*Config, error) {
	config := &Config{
		Zones:    []Zone{},
		ACLs:     []ACL{},
		Keys:     []Key{},
		TLS:      []TLSConfig{},
		Includes: []string{},
		Masters:  []Masters{},
		Servers:  []Server{},
		Views:    []View{},
		Unknown:  []UnknownStatement{},
		Options: Options{
			Additional: make(map[string]string),
		},
	}

	for p.nextLine() {
		if err := p.parseStatement(config); err != nil {
			return nil, fmt.Errorf("line %d: %v", p.line, err)
		}
	}

	return config, nil
}

// nextLine reads the next non-empty, non-comment line
func (p *Parser) nextLine() bool {
	var content strings.Builder
	inBlockComment := false

	for p.scanner.Scan() {
		p.line++
		line := p.scanner.Text()

		// Handle block comments (/* ... */)
		for {
			if inBlockComment {
				if idx := strings.Index(line, "*/"); idx != -1 {
					line = line[idx+2:]
					inBlockComment = false
				} else {
					line = ""
					break
				}
			} else {
				if idx := strings.Index(line, "/*"); idx != -1 {
					before := line[:idx]
					after := line[idx+2:]
					if endIdx := strings.Index(after, "*/"); endIdx != -1 {
						line = before + after[endIdx+2:]
					} else {
						line = before
						inBlockComment = true
					}
				} else {
					break
				}
			}
		}

		if inBlockComment {
			continue
		}

		line = strings.TrimSpace(line)

		// Skip empty lines and full-line comments
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove inline comments
		if idx := strings.Index(line, "//"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
			if line == "" {
				continue
			}
		}
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
			if line == "" {
				continue
			}
		}

		content.WriteString(line + " ")

		// Check if we have a complete statement (ends with ; or })
		trimmed := strings.TrimSpace(content.String())
		if strings.HasSuffix(trimmed, ";") || strings.HasSuffix(trimmed, "}") ||
			strings.Contains(trimmed, "{") {
			p.current = trimmed
			p.tokenize()
			p.pos = 0
			return true
		}
	}

	// Handle any remaining content
	if content.Len() > 0 {
		p.current = strings.TrimSpace(content.String())
		p.tokenize()
		p.pos = 0
		return true
	}

	return false
}

// tokenize splits the current line into tokens
func (p *Parser) tokenize() {
	p.tokens = []string{}
	var current strings.Builder
	inQuotes := false
	escapeNext := false

	for _, r := range p.current {
		if escapeNext {
			current.WriteRune(r)
			escapeNext = false
			continue
		}

		if r == '\\' {
			escapeNext = true
			continue
		}

		if r == '"' {
			inQuotes = !inQuotes
			current.WriteRune(r)
			continue
		}

		if !inQuotes && (unicode.IsSpace(r) || r == ';' || r == '{' || r == '}') {
			if current.Len() > 0 {
				p.tokens = append(p.tokens, current.String())
				current.Reset()
			}
			if r == ';' || r == '{' || r == '}' {
				p.tokens = append(p.tokens, string(r))
			}
			continue
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		p.tokens = append(p.tokens, current.String())
	}
}

// peek returns the next token without consuming it
func (p *Parser) peek() string {
	if p.pos >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.pos]
}

// next returns and consumes the next token
func (p *Parser) next() string {
	if p.pos >= len(p.tokens) {
		return ""
	}
	token := p.tokens[p.pos]
	p.pos++
	return token
}

// parseStatement parses a top-level statement
func (p *Parser) parseStatement(config *Config) error {
	token := p.peek()
	if token == "" {
		return nil
	}

	switch token {
	case "zone":
		zone, err := p.parseZone()
		if err != nil {
			return err
		}
		config.Zones = append(config.Zones, *zone)
	case "options":
		return p.parseOptions(&config.Options)
	case "acl":
		acl, err := p.parseACL()
		if err != nil {
			return err
		}
		config.ACLs = append(config.ACLs, *acl)
	case "key":
		key, err := p.parseKey()
		if err != nil {
			return err
		}
		config.Keys = append(config.Keys, *key)
	case "tls":
		tls, err := p.parseTLS()
		if err != nil {
			return err
		}
		config.TLS = append(config.TLS, *tls)
	case "logging":
		logging, err := p.parseLogging()
		if err != nil {
			return err
		}
		config.Logging = logging
	case "controls":
		controls, err := p.parseControls()
		if err != nil {
			return err
		}
		config.Controls = controls
	case "include":
		include, err := p.parseInclude()
		if err != nil {
			return err
		}
		config.Includes = append(config.Includes, include)
	case "masters":
		masters, err := p.parseMasters()
		if err != nil {
			return err
		}
		config.Masters = append(config.Masters, *masters)
	case "server":
		server, err := p.parseServer()
		if err != nil {
			return err
		}
		config.Servers = append(config.Servers, *server)
	case "view":
		view, err := p.parseView()
		if err != nil {
			return err
		}
		config.Views = append(config.Views, *view)
	case "statistics-channels":
		stats, err := p.parseStatistics()
		if err != nil {
			return err
		}
		config.Statistics = stats
	default:
		// Handle unknown statements
		unknown := p.parseUnknownStatement()
		config.Unknown = append(config.Unknown, unknown)
	}

	return nil
}

// parseZone parses a zone statement
func (p *Parser) parseZone() (*Zone, error) {
	p.next() // consume "zone"

	zoneName := p.next()
	if zoneName == "" {
		return nil, fmt.Errorf("expected zone name")
	}

	// Remove quotes if present
	zoneName = strings.Trim(zoneName, "\"")

	// Optional class (IN, CH, HS, etc.)
	nextToken := p.peek()
	if nextToken != "{" {
		// This could be a class - consume it
		class := p.next()
		// Common classes: IN, CH, HS
		if strings.ToUpper(class) == "IN" || strings.ToUpper(class) == "CH" || strings.ToUpper(class) == "HS" {
			// Valid class, continue to expect '{'
		} else {
			// Not a recognized class, might be something else - put it back
			p.pos-- // step back
		}
	}

	if p.next() != "{" {
		return nil, fmt.Errorf("expected '{' after zone name")
	}

	zone := &Zone{
		Name:    zoneName,
		Options: make(map[string]string),
	}

	// Parse zone body
	for p.nextLine() {
		token := p.peek()
		if token == "}" {
			p.next()
			if p.peek() == ";" {
				p.next()
			}
			break
		}

		switch token {
		case "type":
			p.next()
			zone.Type = p.next()
			p.expectSemicolon()
		case "file":
			p.next()
			zone.File = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "notify":
			p.next()
			val := p.next()
			notify := val == "yes"
			zone.Notify = &notify
			p.expectSemicolon()
		case "dnssec-policy":
			p.next()
			zone.DNSSECPolicy = p.next()
			p.expectSemicolon()
		case "inline-signing":
			p.next()
			val := p.next()
			inlineSigning := val == "yes"
			zone.InlineSigning = &inlineSigning
			p.expectSemicolon()
		case "allow-transfer":
			p.next()
			if p.next() != "{" {
				return nil, fmt.Errorf("expected '{' after allow-transfer")
			}
			for {
				addr := p.next()
				if addr == "}" {
					break
				}
				zone.AllowTransfer = append(zone.AllowTransfer, strings.Trim(addr, ";"))
			}
			p.expectSemicolon()
		case "masters":
			p.next()
			if p.next() != "{" {
				return nil, fmt.Errorf("expected '{' after masters")
			}
			for {
				master := p.next()
				if master == "}" {
					break
				}
				zone.Masters = append(zone.Masters, strings.Trim(master, ";"))
			}
			p.expectSemicolon()
		default:
			// Generic option
			key := p.next()
			value := p.next()
			zone.Options[key] = strings.Trim(value, "\";")
			p.expectSemicolon()
		}
	}

	return zone, nil
}

// parseOptions parses the options block
func (p *Parser) parseOptions(options *Options) error {
	p.next() // consume "options"

	if p.next() != "{" {
		return fmt.Errorf("expected '{' after options")
	}

	for p.nextLine() {
		token := p.peek()
		if token == "}" {
			p.next()
			if p.peek() == ";" {
				p.next()
			}
			break
		}

		switch token {
		case "directory":
			p.next()
			options.Directory = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "pid-file":
			p.next()
			options.PidFile = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "listen-on", "listen-on-v6":
			listenType := token
			p.next()

			listenOn := ListenOn{Port: 53} // default port

			// Parse optional port
			if p.peek() == "port" {
				p.next() // consume "port"
				portStr := p.next()
				if port, err := strconv.Atoi(portStr); err == nil {
					listenOn.Port = port
				}
			}

			// Parse optional TLS
			if p.peek() == "tls" {
				p.next() // consume "tls"
				listenOn.TLS = p.next()
			}

			// Parse optional HTTP
			if p.peek() == "http" {
				p.next() // consume "http"
				listenOn.HTTP = p.next()
			}

			// Parse addresses block
			if p.next() != "{" {
				return fmt.Errorf("expected '{' after listen-on")
			}

			for {
				addr := p.next()
				if addr == "}" {
					break
				}
				listenOn.Addresses = append(listenOn.Addresses, strings.Trim(addr, ";"))
			}

			if listenType == "listen-on" {
				options.ListenOn = append(options.ListenOn, listenOn)
			} else {
				options.ListenOnV6 = append(options.ListenOnV6, listenOn)
			}
			p.expectSemicolon()
		case "allow-new-zones":
			p.next()
			val := p.next()
			allowNewZones := val == "yes"
			options.AllowNewZones = &allowNewZones
			p.expectSemicolon()
		case "allow-query-cache":
			p.next()
			if p.next() != "{" {
				return fmt.Errorf("expected '{' after allow-query-cache")
			}
			for {
				entry := p.next()
				if entry == "}" {
					break
				}
				options.AllowQueryCache = append(options.AllowQueryCache, strings.Trim(entry, ";"))
			}
			p.expectSemicolon()
		case "also-notify":
			p.next()
			if p.next() != "{" {
				return fmt.Errorf("expected '{' after also-notify")
			}
			for {
				entry := p.next()
				if entry == "}" {
					break
				}
				options.AlsoNotify = append(options.AlsoNotify, strings.Trim(entry, ";"))
			}
			p.expectSemicolon()
		case "dump-file":
			p.next()
			options.DumpFile = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "statistics-file":
			p.next()
			options.StatisticsFile = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "memstatistics-file":
			p.next()
			options.MemstatisticsFile = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "secroots-file":
			p.next()
			options.SecrootsFile = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "recursing-file":
			p.next()
			options.RecursingFile = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "managed-keys-directory":
			p.next()
			options.ManagedKeysDirectory = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "geoip-directory":
			p.next()
			options.GeoipDirectory = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "session-keyfile":
			p.next()
			options.SessionKeyfile = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "allow-query":
			p.next()
			if p.next() != "{" {
				return fmt.Errorf("expected '{' after allow-query")
			}
			for {
				entry := p.next()
				if entry == "}" {
					break
				}
				options.AllowQuery = append(options.AllowQuery, strings.Trim(entry, ";"))
			}
			p.expectSemicolon()

		case "forwarders":
			p.next()
			if p.next() != "{" {
				return fmt.Errorf("expected '{' after forwarders")
			}
			for {
				forwarder := p.next()
				if forwarder == "}" {
					break
				}
				options.Forwarders = append(options.Forwarders, strings.Trim(forwarder, ";"))
			}
			p.expectSemicolon()
		case "forward":
			p.next()
			options.Forward = p.next()
			p.expectSemicolon()
		case "recursion":
			p.next()
			val := p.next()
			recursion := val == "yes"
			options.Recursion = &recursion
			p.expectSemicolon()
		case "version":
			p.next()
			options.Version = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "hostname":
			p.next()
			options.Hostname = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "server-id":
			p.next()
			options.ServerID = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "dnssec-validation":
			p.next()
			options.DNSSECValidation = p.next()
			p.expectSemicolon()
		default:
			// Generic option
			key := p.next()
			value := p.next()
			options.Additional[key] = strings.Trim(value, "\";")
			p.expectSemicolon()
		}
	}

	return nil
}

// parseACL parses an ACL definition
func (p *Parser) parseACL() (*ACL, error) {
	p.next() // consume "acl"

	name := strings.Trim(p.next(), "\"")
	if p.next() != "{" {
		return nil, fmt.Errorf("expected '{' after ACL name")
	}

	acl := &ACL{
		Name:    name,
		Entries: []string{},
	}

	for p.nextLine() {
		token := p.peek()
		if token == "}" {
			p.next()
			if p.peek() == ";" {
				p.next()
			}
			break
		}

		entry := p.next()
		acl.Entries = append(acl.Entries, strings.Trim(entry, ";"))
	}

	return acl, nil
}

// parseKey parses a key definition
func (p *Parser) parseKey() (*Key, error) {
	p.next() // consume "key"

	name := strings.Trim(p.next(), "\"")
	if p.next() != "{" {
		return nil, fmt.Errorf("expected '{' after key name")
	}

	key := &Key{Name: name}

	for p.nextLine() {
		token := p.peek()
		if token == "}" {
			p.next()
			if p.peek() == ";" {
				p.next()
			}
			break
		}

		switch token {
		case "algorithm":
			p.next()
			key.Algorithm = p.next()
			p.expectSemicolon()
		case "secret":
			p.next()
			key.Secret = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		}
	}

	return key, nil
}

// parseTLS parses a TLS configuration block
func (p *Parser) parseTLS() (*TLSConfig, error) {
	p.next() // consume "tls"

	name := p.next()
	if p.next() != "{" {
		return nil, fmt.Errorf("expected '{' after tls name")
	}

	tls := &TLSConfig{Name: name}

	for p.nextLine() {
		token := p.peek()
		if token == "}" {
			p.next()
			if p.peek() == ";" {
				p.next()
			}
			break
		}

		switch token {
		case "cert-file":
			p.next()
			tls.CertFile = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		case "key-file":
			p.next()
			tls.KeyFile = strings.Trim(p.next(), "\"")
			p.expectSemicolon()
		}
	}

	return tls, nil
}

// parseLogging parses the logging block (simplified)
func (p *Parser) parseLogging() (*Logging, error) {
	p.next() // consume "logging"

	if p.next() != "{" {
		return nil, fmt.Errorf("expected '{' after logging")
	}

	logging := &Logging{
		Channels:   []LogChannel{},
		Categories: []LogCategory{},
	}

	// This is a simplified implementation
	// Full logging parsing would be much more complex
	for p.nextLine() {
		token := p.peek()
		if token == "}" {
			p.next()
			if p.peek() == ";" {
				p.next()
			}
			break
		}
		// Skip logging content for now
		p.next()
	}

	return logging, nil
}

// parseControls parses the controls block (simplified)
func (p *Parser) parseControls() (*Controls, error) {
	p.next() // consume "controls"

	if p.next() != "{" {
		return nil, fmt.Errorf("expected '{' after controls")
	}

	controls := &Controls{
		Inet: []ControlInet{},
		Unix: []ControlUnix{},
	}

	// Simplified implementation
	for p.nextLine() {
		token := p.peek()
		if token == "}" {
			p.next()
			if p.peek() == ";" {
				p.next()
			}
			break
		}
		p.next()
	}

	return controls, nil
}

// parseInclude parses an include statement
func (p *Parser) parseInclude() (string, error) {
	p.next() // consume "include"

	filename := strings.Trim(p.next(), "\"")
	p.expectSemicolon()

	return filename, nil
}

// parseMasters parses a masters definition
func (p *Parser) parseMasters() (*Masters, error) {
	p.next() // consume "masters"

	name := strings.Trim(p.next(), "\"")
	if p.next() != "{" {
		return nil, fmt.Errorf("expected '{' after masters name")
	}

	masters := &Masters{
		Name:    name,
		Masters: []string{},
	}

	for p.nextLine() {
		token := p.peek()
		if token == "}" {
			p.next()
			if p.peek() == ";" {
				p.next()
			}
			break
		}

		master := p.next()
		masters.Masters = append(masters.Masters, strings.Trim(master, ";"))
	}

	return masters, nil
}

// parseServer parses a server statement
func (p *Parser) parseServer() (*Server, error) {
	p.next() // consume "server"

	address := p.next()
	if p.next() != "{" {
		return nil, fmt.Errorf("expected '{' after server address")
	}

	server := &Server{
		Address:    address,
		Additional: make(map[string]string),
	}

	for p.nextLine() {
		token := p.peek()
		if token == "}" {
			p.next()
			if p.peek() == ";" {
				p.next()
			}
			break
		}

		// Simplified - just store as additional options
		key := p.next()
		value := p.next()
		server.Additional[key] = strings.Trim(value, "\";")
		p.expectSemicolon()
	}

	return server, nil
}

// parseView parses a view statement (simplified)
func (p *Parser) parseView() (*View, error) {
	p.next() // consume "view"

	name := strings.Trim(p.next(), "\"")

	// Optional class
	class := ""
	if p.peek() != "{" {
		class = p.next()
	}

	if p.next() != "{" {
		return nil, fmt.Errorf("expected '{' after view name")
	}

	view := &View{
		Name:    name,
		Class:   class,
		Zones:   []Zone{},
		Options: make(map[string]string),
	}

	// Simplified view parsing
	for p.nextLine() {
		token := p.peek()
		if token == "}" {
			p.next()
			if p.peek() == ";" {
				p.next()
			}
			break
		}

		if token == "zone" {
			zone, err := p.parseZone()
			if err != nil {
				return nil, err
			}
			view.Zones = append(view.Zones, *zone)
		} else {
			// Generic option
			key := p.next()
			value := p.next()
			view.Options[key] = strings.Trim(value, "\";")
			p.expectSemicolon()
		}
	}

	return view, nil
}

// parseStatistics parses statistics-channels (simplified)
func (p *Parser) parseStatistics() (*Statistics, error) {
	p.next() // consume "statistics-channels"

	if p.next() != "{" {
		return nil, fmt.Errorf("expected '{' after statistics-channels")
	}

	stats := &Statistics{
		Channels: []StatChannel{},
	}

	// Simplified implementation
	for p.nextLine() {
		token := p.peek()
		if token == "}" {
			p.next()
			if p.peek() == ";" {
				p.next()
			}
			break
		}
		p.next()
	}

	return stats, nil
}

// parseUnknownStatement parses unknown statements
func (p *Parser) parseUnknownStatement() UnknownStatement {
	stmt := UnknownStatement{
		Type:    p.next(),
		Content: strings.Join(p.tokens[p.pos:], " "),
	}
	p.pos = len(p.tokens) // consume all tokens
	return stmt
}

// expectSemicolon consumes a semicolon if present
func (p *Parser) expectSemicolon() {
	if p.peek() == ";" {
		p.next()
	}
}

// ParseString is a convenience function to parse a named.conf from a string
func ParseString(content string) (*Config, error) {
	return NewParser(strings.NewReader(content)).Parse()
}

// Example usage and helper functions

// String returns a string representation of the config (for debugging)
func (c *Config) String() string {
	var sb strings.Builder

	if len(c.TLS) > 0 {
		sb.WriteString(fmt.Sprintf("TLS Configurations: %d\n", len(c.TLS)))
		for _, tls := range c.TLS {
			sb.WriteString(fmt.Sprintf("  - %s (cert: %s, key: %s)\n", tls.Name, tls.CertFile, tls.KeyFile))
		}
	}

	if len(c.Zones) > 0 {
		sb.WriteString(fmt.Sprintf("Zones: %d\n", len(c.Zones)))
		for _, zone := range c.Zones {
			sb.WriteString(fmt.Sprintf("  - %s (%s)\n", zone.Name, zone.Type))
		}
	}

	if len(c.ACLs) > 0 {
		sb.WriteString(fmt.Sprintf("ACLs: %d\n", len(c.ACLs)))
		for _, acl := range c.ACLs {
			sb.WriteString(fmt.Sprintf("  - %s (%d entries)\n", acl.Name, len(acl.Entries)))
		}
	}

	if len(c.Keys) > 0 {
		sb.WriteString(fmt.Sprintf("Keys: %d\n", len(c.Keys)))
	}

	if c.Options.Directory != "" {
		sb.WriteString(fmt.Sprintf("Directory: %s\n", c.Options.Directory))
	}

	if len(c.Options.ListenOn) > 0 {
		sb.WriteString(fmt.Sprintf("Listen-On Interfaces: %d\n", len(c.Options.ListenOn)))
		for _, listen := range c.Options.ListenOn {
			sb.WriteString(fmt.Sprintf("  - Port %d", listen.Port))
			if listen.TLS != "" {
				sb.WriteString(fmt.Sprintf(" (TLS: %s)", listen.TLS))
			}
			if listen.HTTP != "" {
				sb.WriteString(fmt.Sprintf(" (HTTP: %s)", listen.HTTP))
			}
			sb.WriteString(fmt.Sprintf(" - %v\n", listen.Addresses))
		}
	}

	return sb.String()
}
