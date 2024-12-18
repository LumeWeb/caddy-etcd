package etcd

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"path"
	"strings"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestPipeline(t *testing.T) {
	var arr []int
	push := func(n int, shouldErr bool) backoff.Operation {
		return func() error {
			if shouldErr {
				return errors.New("push error")
			}
			arr = append(arr, n)
			return nil
		}
	}
	pop := func() backoff.Operation {
		return func() error {
			arr = arr[0 : len(arr)-1]
			return nil
		}
	}
	noop := func() backoff.Operation {
		return func() error {
			return nil
		}
	}

	var tcs = []struct {
		Commit    []backoff.Operation
		Rollback  []backoff.Operation
		ShouldErr bool
		Expect    []int
	}{
		{Commit: tx(push(1, false), push(2, false)), Rollback: nil, ShouldErr: false, Expect: []int{1, 2}},
		{Commit: tx(push(1, false), push(2, true)), Rollback: tx(pop(), pop()), ShouldErr: true, Expect: []int{}},
		{Commit: tx(push(1, false), push(2, true)), Rollback: tx(pop()), ShouldErr: true, Expect: []int{}},
		{Commit: tx(push(1, false), push(2, true)), Rollback: nil, ShouldErr: true, Expect: []int{1}},
		{Commit: tx(push(1, false), push(2, false), push(3, true)), Rollback: tx(pop(), noop(), pop()), ShouldErr: true, Expect: []int{1}},
	}
	for _, tc := range tcs {
		arr = []int{}
		err := pipeline(tc.Commit, tc.Rollback, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 1))
		switch tc.ShouldErr {
		case true:
			assert.Error(t, err)
		default:
			assert.NoError(t, err)
		}
		assert.Equal(t, tc.Expect, arr)
	}
}

func TestLowLevelSet(t *testing.T) {
	if !shouldRunIntegration() {
		t.Skip("no etcd server found, skipping")
	}
	h := newTestHelper(t)
	defer h.cleanup()
	tcs := []struct {
		Path  string
		Value []byte
	}{
		{Path: "test", Value: []byte("test")},
		{Path: "/test", Value: []byte("test")},
		{Path: "/deeply/nested/value.md", Value: []byte("test")},
	}
	for _, tc := range tcs {
		errC := set(h.client, path.Join(h.cfg.KeyPrefix, tc.Path), tc.Value)()
		assert.NoError(t, errC)

		// Verify using Get
		resp, err := h.client.Get(context.Background(), path.Join(h.cfg.KeyPrefix, tc.Path))
		assert.NoError(t, err)
		assert.Equal(t, 1, len(resp.Kvs))
		assert.Equal(t, path.Join(h.cfg.KeyPrefix, tc.Path), string(resp.Kvs[0].Key))
		assert.Equal(t, base64.StdEncoding.EncodeToString(tc.Value), string(resp.Kvs[0].Value))
	}
}

func TestLowLevelGet(t *testing.T) {
    if !shouldRunIntegration() {
        t.Skip("no etcd server found, skipping")
    }
    h := newTestHelper(t)
    defer h.cleanup()

    tcs := []struct {
        Path  string
        Value []byte
    }{
        {Path: "test", Value: []byte("test")},
        {Path: "/test", Value: []byte("test")},
        {Path: "/deeply/nested/value", Value: []byte("test")},
    }
    
    // Use the test helper's client instead of creating new ones
    for _, tc := range tcs {
        // Set value first
        err := set(h.client, path.Join(h.cfg.KeyPrefix, tc.Path), tc.Value)()
        assert.NoError(t, err)

        // Test get operation
        var buf bytes.Buffer
        errC := get(h.client, path.Join(h.cfg.KeyPrefix, tc.Path), &buf)()
        assert.NoError(t, errC)
        assert.Equal(t, tc.Value, buf.Bytes())

        // Test non-existent key
        var emptyBuf bytes.Buffer
        errD := get(h.client, path.Join(h.cfg.KeyPrefix, "nonexistent"), &emptyBuf)()
        assert.NoError(t, errD)
        assert.Equal(t, 0, emptyBuf.Len())
    }
}

func TestLowLevelMD(t *testing.T) {
	if !shouldRunIntegration() {
		t.Skip("no etcd server found, skipping")
	}
	h := newTestHelper(t)
	defer h.cleanup()

	data := []byte("test data")
	expSum := sha1.Sum(data)
	p := "/testmd/some/path/key.md"
	key := path.Join(h.cfg.KeyPrefix, p)
	md := NewMetadata(p, data)

	assert.Equal(t, p, md.Path)
	assert.Equal(t, expSum, md.Hash)
	assert.Equal(t, len(data), md.Size)

	err := setMD(h.client, key, md)()
	assert.NoError(t, err)

	var md2 Metadata
	err = getMD(h.client, key, &md2)()
	assert.NoError(t, err)

	assert.Equal(t, md, md2)
	assert.Equal(t, expSum, md2.Hash)
	assert.Equal(t, len(data), md2.Size)
	assert.Equal(t, p, md2.Path)
}

func TestListLowLevel(t *testing.T) {
	if !shouldRunIntegration() {
		t.Skip("no etcd server found, skipping")
	}
	h := newTestHelper(t)
	defer h.cleanup()

	paths := []string{
		"/one/two/three.end",
		"/one/two/four.end",
		"/one/five/six/seven.end",
		"/one/five/eleven.end",
		"/one/five/six/ten.end",
	}

	for _, p := range paths {
		err := set(h.client, path.Join(h.cfg.KeyPrefix, p), []byte("test"))()
		assert.NoError(t, err)
	}

	out, err := list(h.client, path.Join(h.cfg.KeyPrefix, "one"))
	assert.NoError(t, err)

	var s []string
	for _, n := range out {
		s = append(s, strings.TrimPrefix(string(n.Key), h.cfg.KeyPrefix))
	}

	for _, p := range paths {
		assert.Contains(t, s, p)
	}
}
