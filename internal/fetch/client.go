package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/marlliton/goinvest/internal/norm"
	"golang.org/x/time/rate"
)

const maxAttempts = 3

// rate.Every(0) devolve rate.Inf: sem default, um Config zero-valor iria anônimo e sem limite.
const (
	defaultUserAgent = "goinvest/0.1 (+https://github.com/marlliton/goinvest) uso pessoal"
	defaultRateEvery = 2 * time.Second
)

type Cache interface {
	Get(ctx context.Context, url string) (body []byte, fetchedAt time.Time, found bool, err error)
	Put(ctx context.Context, url, docKind string, body []byte, fetchedAt time.Time) error
}

type Config struct {
	UserAgent string
	RateEvery time.Duration
	Cache     Cache
	Now       func() time.Time
}

type Client struct {
	httpClient *http.Client
	limiter    *rate.Limiter
	cfg        Config
}

func NewClient(cfg Config) *Client {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}
	if cfg.RateEvery <= 0 {
		cfg.RateEvery = defaultRateEvery
	}
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		limiter:    rate.NewLimiter(rate.Every(cfg.RateEvery), 1),
		cfg:        cfg,
	}
}

// Get devolve o corpo já em UTF-8.
func (c *Client) Get(ctx context.Context, url, docKind string, ttl time.Duration, force bool) ([]byte, error) {
	return c.get(ctx, url, docKind, ttl, force, decodeLatin1)
}

// Fonte que já serve UTF-8, ou binário, seria corrompida pela decodificação ISO-8859-1.
func (c *Client) GetRaw(ctx context.Context, url, docKind string, ttl time.Duration, force bool) ([]byte, error) {
	return c.get(ctx, url, docKind, ttl, force, readAll)
}

type bodyReader func(io.Reader) ([]byte, error)

func decodeLatin1(r io.Reader) ([]byte, error) {
	decoded, err := norm.DecodeISO88591(r)
	if err != nil {
		return nil, backoff.Permanent(err)
	}
	return io.ReadAll(decoded)
}

func readAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }

func (c *Client) get(ctx context.Context, url, docKind string, ttl time.Duration, force bool, read bodyReader) ([]byte, error) {
	if c.cfg.Cache != nil && !force {
		body, fetchedAt, found, err := c.cfg.Cache.Get(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("cache get %s: %w", url, err)
		}
		if found && c.cfg.Now().Sub(fetchedAt) < ttl {
			return body, nil
		}
	}

	body, err := backoff.Retry(ctx, func() ([]byte, error) { return c.attempt(ctx, url, read) },
		backoff.WithMaxTries(maxAttempts),
		backoff.WithMaxElapsedTime(2*time.Minute))
	if err != nil {
		return nil, err
	}

	if c.cfg.Cache != nil {
		if err := c.cfg.Cache.Put(ctx, url, docKind, body, c.cfg.Now()); err != nil {
			return nil, fmt.Errorf("cache put %s: %w", url, err)
		}
	}
	return body, nil
}

func (c *Client) attempt(ctx context.Context, url string, read bodyReader) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, backoff.Permanent(fmt.Errorf("rate limiter %s: %w", url, err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, backoff.Permanent(fmt.Errorf("build request %s: %w", url, err))
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if err := statusError(url, resp.StatusCode); err != nil {
		return nil, err
	}

	body, err := read(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body %s: %w", url, err)
	}
	return body, nil
}

// 4xx não melhora com retry; 429 é a exceção, porque a fonte está pedindo para esperar.
func statusError(url string, code int) error {
	switch {
	case code >= 200 && code < 300:
		return nil
	case code == http.StatusTooManyRequests || code >= 500:
		return fmt.Errorf("get %s: status %d", url, code)
	default:
		return backoff.Permanent(fmt.Errorf("get %s: status %d", url, code))
	}
}
