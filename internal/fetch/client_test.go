package fetch_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/stretchr/testify/require"
)

const testRateEvery = 50 * time.Millisecond

type cacheEntry struct {
	body      []byte
	docKind   string
	fetchedAt time.Time
}

type fakeCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	puts    int
}

func newFakeCache() *fakeCache {
	return &fakeCache{entries: map[string]cacheEntry{}}
}

func (c *fakeCache) Get(_ context.Context, url string) ([]byte, time.Time, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[url]
	if !ok {
		return nil, time.Time{}, false, nil
	}
	return e.body, e.fetchedAt, true, nil
}

func (c *fakeCache) Put(_ context.Context, url, docKind string, body []byte, fetchedAt time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[url] = cacheEntry{body: body, docKind: docKind, fetchedAt: fetchedAt}
	c.puts++
	return nil
}

func TestGetSendsOwnUserAgentAndSpacesRequests(t *testing.T) {
	var (
		mu         sync.Mutex
		userAgents []string
		times      []time.Time
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		userAgents = append(userAgents, r.UserAgent())
		times = append(times, time.Now())
		mu.Unlock()
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	for range 3 {
		_, err := c.Get(t.Context(), srv.URL, "test", time.Hour, false)
		require.NoError(t, err)
	}

	require.Len(t, userAgents, 3)
	for _, ua := range userAgents {
		require.Contains(t, ua, "goinvest")
		require.NotContains(t, ua, "Mozilla", "nunca forjar navegador")
	}

	for i := 1; i < len(times); i++ {
		gap := times[i].Sub(times[i-1])
		require.GreaterOrEqual(t, gap, testRateEvery*8/10,
			"requisição %d veio cedo demais: %v", i, gap)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("recuperado"))
	}))
	defer srv.Close()

	c := fetch.NewClient(fetch.Config{RateEvery: time.Millisecond})
	body, err := c.Get(t.Context(), srv.URL, "test", time.Hour, false)
	require.NoError(t, err)
	require.Equal(t, "recuperado", string(body))
	require.Equal(t, int32(2), hits.Load())
}

func TestGetNeverRetriesOn404(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := fetch.NewClient(fetch.Config{RateEvery: time.Millisecond})
	_, err := c.Get(t.Context(), srv.URL, "test", time.Hour, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "404")
	require.Equal(t, int32(1), hits.Load(), "4xx não melhora com retry")
}

func TestGetServesFromCacheWithinTTL(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Write([]byte("da rede"))
	}))
	defer srv.Close()

	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	cache := newFakeCache()
	cache.entries[srv.URL] = cacheEntry{body: []byte("do cache"), fetchedAt: now.Add(-10 * time.Minute)}

	c := fetch.NewClient(fetch.Config{
		RateEvery: time.Millisecond,
		Cache:     cache,
		Now:       func() time.Time { return now },
	})

	body, err := c.Get(t.Context(), srv.URL, "test", time.Hour, false)
	require.NoError(t, err)
	require.Equal(t, "do cache", string(body))
	require.Zero(t, hits.Load(), "dentro do TTL não toca a rede")
}

func TestGetRefetchesWhenCacheIsStaleOrForced(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Write([]byte("da rede"))
	}))
	defer srv.Close()

	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	cache := newFakeCache()
	cache.entries[srv.URL] = cacheEntry{body: []byte("do cache"), fetchedAt: now.Add(-10 * time.Minute)}

	c := fetch.NewClient(fetch.Config{
		RateEvery: time.Millisecond,
		Cache:     cache,
		Now:       func() time.Time { return now },
	})

	body, err := c.Get(t.Context(), srv.URL, "test", time.Minute, false)
	require.NoError(t, err)
	require.Equal(t, "da rede", string(body), "fora do TTL vai à rede")

	body, err = c.Get(t.Context(), srv.URL, "test", time.Hour, true)
	require.NoError(t, err)
	require.Equal(t, "da rede", string(body), "force ignora o cache válido")

	require.Equal(t, int32(2), hits.Load())
	require.Equal(t, 2, cache.puts, "toda ida à rede realimenta o cache")
}

func TestGetDecodesISO88591(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// "Ações" e "Vacância Média" em latin-1: os bytes que o Fundamentus manda.
		w.Write([]byte{0x41, 0xE7, 0xF5, 0x65, 0x73}) // A ç õ e s
	}))
	defer srv.Close()

	c := fetch.NewClient(fetch.Config{RateEvery: time.Millisecond})
	body, err := c.Get(t.Context(), srv.URL, "test", time.Hour, false)
	require.NoError(t, err)
	require.Equal(t, "Ações", string(body))
}
