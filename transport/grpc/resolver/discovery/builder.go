package discovery

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/registry"

	"github.com/google/uuid"
	"google.golang.org/grpc/resolver"
)

const name = "discovery"

// Option is builder option.
type Option func(o *builder)

// WithTimeout with timeout option.
func WithTimeout(timeout time.Duration) Option {
	return func(b *builder) {
		b.timeout = timeout
	}
}

// WithInsecure with isSecure option.
func WithInsecure(insecure bool) Option {
	return func(b *builder) {
		b.insecure = insecure
	}
}

// WithInsecure with isSecure option.
func WithSubset(size int) Option {
	return func(b *builder) {
		b.subsetSize = size
	}
}

// DisableDebugLog disables update instances log.
func DisableDebugLog() Option {
	return func(b *builder) {
		b.debugLogDisabled = true
	}
}

type builder struct {
	discoverer       registry.Discovery
	timeout          time.Duration
	insecure         bool
	subsetSize       int
	debugLogDisabled bool
}

// NewBuilder creates a builder which is used to factory registry resolvers.
func NewBuilder(d registry.Discovery, opts ...Option) resolver.Builder {
	b := &builder{
		discoverer:       d,
		timeout:          time.Second * 10,
		insecure:         false,
		debugLogDisabled: false,
		subsetSize:       25,
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

func (b *builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	watchRes := &struct {
		err error
		w   registry.Watcher
	}{}

	done := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		w, err := b.discoverer.Watch(ctx, strings.TrimPrefix(target.URL.Path, "/"))
		watchRes.w = w
		watchRes.err = err
		close(done)
	}()

	var err error
	select {
	case <-done:
		err = watchRes.err
	case <-time.After(b.timeout):
		err = errors.New("discovery create watcher overtime")
	}
	if err != nil {
		cancel()
		return nil, err
	}

	r := &discoveryResolver{
		w:                watchRes.w,
		cc:               cc,
		ctx:              ctx,
		cancel:           cancel,
		insecure:         b.insecure,
		debugLogDisabled: b.debugLogDisabled,
		subsetSize:       b.subsetSize,
		selecterKey:      uuid.New().String(),
	}
	go r.watch()
	return r, nil
}

// Scheme return scheme of discovery
func (*builder) Scheme() string {
	return name
}
