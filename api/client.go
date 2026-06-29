package api

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

// Verbosity controls the level of request/response logging.
type Verbosity int

const (
	VerbOff    Verbosity = 0
	VerbBasic  Verbosity = 1 // -v: log method + URL + status
	VerbDetail Verbosity = 2 // -vv: also log headers and body
)

// ProxmoxError represents an error returned by the Proxmox API.
type ProxmoxError struct {
	StatusCode int
	Status     string
	Errors     map[string]string
	Message    string
}

func (e *ProxmoxError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("proxmox api %s: %s", e.Status, e.Message)
	}
	if len(e.Errors) > 0 {
		parts := make([]string, 0, len(e.Errors))
		for k, v := range e.Errors {
			parts = append(parts, k+": "+v)
		}
		return fmt.Sprintf("proxmox api %s: %s", e.Status, strings.Join(parts, "; "))
	}
	return fmt.Sprintf("proxmox api %s", e.Status)
}

// Response wraps the standard Proxmox JSON envelope { data, errors }.
type Response struct {
	Data   json.RawMessage   `json:"data"`
	Errors map[string]string `json:"errors,omitempty"`
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithInsecure disables TLS certificate verification.
func WithInsecure(insecure bool) ClientOption {
	return func(c *Client) { c.insecure = insecure }
}

// WithCAFile sets a custom CA certificate file for TLS verification.
func WithCAFile(path string) ClientOption {
	return func(c *Client) { c.caFile = path }
}

// WithVerbosity sets request/response logging verbosity.
func WithVerbosity(v Verbosity) ClientOption {
	return func(c *Client) { c.verbosity = v }
}

// WithTokenAuth sets API token authentication (PVEAPIToken header).
func WithTokenAuth(tokenID, tokenSecret string) ClientOption {
	return func(c *Client) {
		c.tokenID = tokenID
		c.tokenSecret = tokenSecret
	}
}

// WithTicketAuth sets cookie-based ticket authentication with CSRF token.
func WithTicketAuth(ticket, csrfToken string) ClientOption {
	return func(c *Client) {
		c.ticket = ticket
		c.csrfToken = csrfToken
	}
}

// WithLogger sets the output writer for verbose logging. Defaults to os.Stderr.
func WithLogger(w io.Writer) ClientOption {
	return func(c *Client) { c.logWriter = w }
}

// WithMaxRetries overrides the default maximum retries for 5xx errors.
func WithMaxRetries(n int) ClientOption {
	return func(c *Client) { c.maxRetries = n }
}

const (
	defaultMaxRetries = 3
	basePath          = "/api2/json"
)

// TicketResult holds the response from POST /access/ticket.
type TicketResult struct {
	Ticket    string `json:"ticket"`
	CSRFToken string `json:"CSRFPreventionToken"`
	Username  string `json:"username"`
}

// Client is an HTTP client for the Proxmox VE API.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	tokenID     string
	tokenSecret string
	ticket      string
	csrfToken   string
	insecure    bool
	caFile      string
	verbosity   Verbosity
	logWriter   io.Writer
	maxRetries  int
}

// NewClient creates a new Proxmox API client.
// baseURL should be scheme://host[:port], e.g. "https://pve.example.com:8006".
func NewClient(baseURL string, opts ...ClientOption) (*Client, error) {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		logWriter:  os.Stderr,
		maxRetries: defaultMaxRetries,
	}
	for _, opt := range opts {
		opt(c)
	}

	tlsCfg := &tls.Config{}

	if c.insecure {
		tlsCfg.InsecureSkipVerify = true
	}

	if c.caFile != "" {
		pem, err := os.ReadFile(c.caFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA file %s: %w", c.caFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("no valid certificates found in %s", c.caFile)
		}
		tlsCfg.RootCAs = pool
	}

	c.httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
		Timeout: 30 * time.Second,
	}

	return c, nil
}

// Do executes an HTTP request against the Proxmox API, retrying on 5xx errors
// with exponential backoff. It returns the decoded response data.
func (c *Client) Do(method, path string, body io.Reader) (json.RawMessage, error) {
	url := c.baseURL + basePath + path

	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			c.logf(VerbBasic, "retrying after %s (attempt %d/%d)", delay, attempt, c.maxRetries)
			time.Sleep(delay)
		}

		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = strings.NewReader(string(bodyBytes))
		}

		req, err := http.NewRequest(method, url, reqBody)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if c.tokenID != "" && c.tokenSecret != "" {
			req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
		} else if c.ticket != "" {
			req.AddCookie(&http.Cookie{Name: "PVEAuthCookie", Value: c.ticket})
			if c.csrfToken != "" && method != "GET" {
				req.Header.Set("CSRFPreventionToken", c.csrfToken)
			}
		}

		c.logf(VerbBasic, "> %s %s", method, url)
		if c.verbosity >= VerbDetail {
			for k, vs := range req.Header {
				for _, v := range vs {
					c.logf(VerbDetail, "> %s: %s", k, v)
				}
			}
			if bodyBytes != nil {
				c.logf(VerbDetail, "> body: %s", string(bodyBytes))
			}
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("executing request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading response body: %w", err)
			continue
		}

		c.logf(VerbBasic, "< %s", resp.Status)
		if c.verbosity >= VerbDetail {
			for k, vs := range resp.Header {
				for _, v := range vs {
					c.logf(VerbDetail, "< %s: %s", k, v)
				}
			}
			c.logf(VerbDetail, "< body: %s", string(respBody))
		}

		// Retry on 5xx server errors.
		if resp.StatusCode >= 500 {
			lastErr = &ProxmoxError{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
			}
			continue
		}

		return c.decodeResponse(resp.StatusCode, resp.Status, respBody)
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", c.maxRetries, lastErr)
}

// Get performs a GET request.
func (c *Client) Get(path string) (json.RawMessage, error) {
	return c.Do("GET", path, nil)
}

// Post performs a POST request.
func (c *Client) Post(path string, body io.Reader) (json.RawMessage, error) {
	return c.Do("POST", path, body)
}

// Put performs a PUT request.
func (c *Client) Put(path string, body io.Reader) (json.RawMessage, error) {
	return c.Do("PUT", path, body)
}

// Delete performs a DELETE request.
func (c *Client) Delete(path string) (json.RawMessage, error) {
	return c.Do("DELETE", path, nil)
}

// Authenticate performs a POST /access/ticket login and returns the ticket
// and CSRF token. It does NOT modify the client's auth state; use
// WithTicketAuth to create a client that uses the returned credentials.
func (c *Client) Authenticate(username, password string) (*TicketResult, error) {
	form := fmt.Sprintf("username=%s&password=%s", username, password)
	url := c.baseURL + basePath + "/access/ticket"

	req, err := http.NewRequest("POST", url, strings.NewReader(form))
	if err != nil {
		return nil, fmt.Errorf("creating auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	c.logf(VerbBasic, "> POST %s", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing auth request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading auth response: %w", err)
	}

	c.logf(VerbBasic, "< %s", resp.Status)

	data, err := c.decodeResponse(resp.StatusCode, resp.Status, respBody)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	var result TicketResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decoding ticket response: %w", err)
	}
	return &result, nil
}

// decodeResponse parses the Proxmox JSON envelope and returns
// the data field or an error.
func (c *Client) decodeResponse(statusCode int, status string, body []byte) (json.RawMessage, error) {
	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		if statusCode >= 400 {
			return nil, &ProxmoxError{
				StatusCode: statusCode,
				Status:     status,
				Message:    string(body),
			}
		}
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if statusCode >= 400 {
		return nil, &ProxmoxError{
			StatusCode: statusCode,
			Status:     status,
			Errors:     resp.Errors,
		}
	}

	return resp.Data, nil
}

func (c *Client) logf(level Verbosity, format string, args ...interface{}) {
	if c.verbosity >= level {
		fmt.Fprintf(c.logWriter, format+"\n", args...)
	}
}
