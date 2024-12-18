package etcd

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"path"
	"strings"
	"time"
)

// token is a random value used to manage locks
var token string

type Service interface {
	Store(key string, value []byte) error
	Load(key string) ([]byte, error)
	Delete(key string) error
	Metadata(key string) (*Metadata, error)
	Lock(key string) error
	Unlock(key string) error
	List(path string, filters ...func(*mvccpb.KeyValue) bool) ([]string, error)
	prefix() string
}

// Lock represents a distributed lock in etcd. Features:
//   - Unique token per client for lock ownership
//   - Automatic lock extension for same client
//   - Configurable lock timeouts
//   - Automatic cleanup of stale locks
//   - Safe concurrent access across cluster
//
// Note: The implementation assumes a single client does not attempt to acquire
// the same lock from multiple goroutines simultaneously. Such usage may result
// in race conditions where the last write wins.
type Lock struct {
	Token    string // Random token identifying the client holding the lock
	Obtained string // UTC timestamp when the lock was obtained
	Key      string // The key being locked
}

// Metadata stores information about a particular node that represents a file in etcd
type Metadata struct {
	Path      string    // Full path to the node
	Size      int       // Size of the value in bytes
	Timestamp time.Time // Last modification time
	Hash      [20]byte  // SHA1 hash of the value
	IsDir     bool      // Whether this node represents a directory
}

// NewMetadata returns metadata information given a path and a file to be stored at the path.
// Typically, one metadata node is stored for each file node in etcd.
func NewMetadata(key string, data []byte) Metadata {
	return Metadata{
		Path:      key,
		Size:      len(data),
		Timestamp: time.Now().UTC(),
		Hash:      sha1.Sum(data),
	}
}

func init() {
	tok := make([]byte, 32)
	_, err := rand.Read(tok)
	if err != nil {
		log.Fatal(err)
	}
	token = base64.StdEncoding.EncodeToString(tok)
}

// NewService returns a new low level service to store and load values in etcd. The service implements
// filesystem-like semantics on top of etcd's key/value storage, with support for:
//   - Atomic transactions for data consistency
//   - Metadata tracking for each stored value
//   - Directory-like operations with recursive listing
//   - Distributed locking with configurable timeouts
//   - Automatic connection management and retries
//   - Data integrity verification via checksums
//
// The service uses exponential backoff for retries and transactions to handle temporary failures.
// While best efforts are made to maintain consistency, prolonged etcd unavailability may impact
// the system's ability to recover to a fully coherent state.
func NewService(c *ClusterConfig) (Service, error) {
	cli, err := getClient(c)
	if err != nil {
		return nil, err
	}

	return &etcdsrv{
		mdPrefix: path.Join(c.KeyPrefix + "/md"),
		lockKey:  path.Join(c.KeyPrefix, "/lock"),
		cfg:      c,
		cli:      cli,
	}, nil
}

type etcdsrv struct {
	mdPrefix  string
	lockKey   string
	cfg       *ClusterConfig
	cli       *clientv3.Client
	noBackoff bool
}

// Lock acquires a lock with a maximum lifetime specified by the ClusterConfig
func (e *etcdsrv) Lock(key string) error {
	return e.lock(token, key)
}

// lock is an internal function that acquires a lock with the given token
func (e *etcdsrv) lock(tok string, key string) error {
	lockKey := path.Join(e.lockKey, key)

	acquire := func() error {
		// Use a longer timeout for lock operations
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := e.cli.Get(ctx, lockKey)
		if err != nil {
			return errors.Wrap(err, "lock: failed to get existing lock")
		}

		var okToSet bool
		if len(resp.Kvs) == 0 {
			okToSet = true
		} else {
			var l Lock
			b, err := base64.StdEncoding.DecodeString(string(resp.Kvs[0].Value))
			if err != nil {
				return errors.Wrap(err, "lock: failed to decode base64 lock representation")
			}
			if err := json.Unmarshal(b, &l); err != nil {
				return errors.Wrap(err, "lock: failed to unmarshal existing lock")
			}
			var lockTime time.Time
			if err := lockTime.UnmarshalText([]byte(l.Obtained)); err != nil {
				return errors.Wrap(err, "lock: failed to unmarshal time")
			}

			// lock request from same client extends existing lock
			if l.Token == tok {
				okToSet = true
			}
			// orphaned locks that are past lock timeout allow new lock
			if time.Now().UTC().Sub(lockTime) >= e.cfg.LockTimeout.Duration {
				okToSet = true
			}
		}

		if okToSet {
			now, err := time.Now().UTC().MarshalText()
			if err != nil {
				return errors.Wrap(err, "lock: failed to marshal current UTC time")
			}
			l := Lock{
				Token:    tok,
				Obtained: string(now),
				Key:      key,
			}
			b, err := json.Marshal(l)
			if err != nil {
				return errors.Wrap(err, "lock: failed to marshal new lock")
			}
			_, err = e.cli.Put(ctx, lockKey, base64.StdEncoding.EncodeToString(b))
			if err != nil {
				return errors.Wrap(err, "failed to get lock")
			}
			return nil
		}
		return LockError{
			Key:     key,
			Timeout: e.cfg.LockTimeout.Duration,
			Err:     fmt.Errorf("lock already exists"),
		}
	}
	return e.execute(acquire)
}

// Unlock releases the lock for the given key
func (e *etcdsrv) Unlock(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Increased timeout for list operations
	defer cancel()

	release := func() error {
		_, err := e.cli.Delete(ctx, path.Join(e.lockKey, key))
		if err != nil {
			return errors.Wrap(err, "failed to release lock")
		}
		return nil
	}
	return e.execute(release)
}

// execute will use exponential backoff when configured
func (e *etcdsrv) execute(o backoff.Operation) error {
	switch e.noBackoff {
	case true:
		return o()
	default:
		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 30 * time.Second
		b.InitialInterval = 100 * time.Millisecond
		b.MaxInterval = 5 * time.Second
		return backoff.Retry(o, b)
	}
}

func (e *etcdsrv) List(key string, filters ...func(*mvccpb.KeyValue) bool) ([]string, error) {
    // Normalize the key to always start with /
    if !strings.HasPrefix(key, "/") {
        key = "/" + key
    }

    // Create the full search path including the etcd prefix
    searchKey := path.Join(e.cfg.KeyPrefix, key)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Get all keys with the prefix
    resp, err := e.cli.Get(ctx, searchKey, clientv3.WithPrefix())
    if err != nil {
        return nil, errors.Wrap(err, "List: failed to get keys")
    }

    var out []string
    prefixLen := len(e.cfg.KeyPrefix)

    // Process and filter the keys
    for _, kv := range resp.Kvs {
        if kv == nil || len(kv.Key) <= prefixLen {
            continue
        }

        // Get the key relative to the prefix
        relativeKey := string(kv.Key)[prefixLen:]

        // Skip metadata entries
        if strings.Contains(relativeKey, "/md/") {
            continue
        }

        // Apply all filters
        skip := false
        for _, filter := range filters {
            if !filter(kv) {
                skip = true
                break
            }
        }
        if skip {
            continue
        }

        // Ensure the key starts with /
        if !strings.HasPrefix(relativeKey, "/") {
            relativeKey = "/" + relativeKey
        }

        out = append(out, relativeKey)
    }

    return out, nil
}

func (e *etcdsrv) Store(key string, value []byte) error {
	storageKey := path.Join(e.cfg.KeyPrefix, key)
	storageKeyMD := path.Join(e.mdPrefix, key)
	md := NewMetadata(key, value)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Increased timeout for store operations
	defer cancel()

	resp, err := e.cli.Get(ctx, storageKeyMD)
	if err != nil {
		return errors.Wrap(err, "store: failed to check metadata")
	}

	txn := e.cli.Txn(ctx)

	if len(resp.Kvs) > 0 {
		mdBytes, err := json.Marshal(md)
		if err != nil {
			return errors.Wrap(err, "store: failed to marshal metadata")
		}
		mdEncoded := base64.StdEncoding.EncodeToString(mdBytes)

		txn.Then(
			clientv3.OpPut(storageKey, base64.StdEncoding.EncodeToString(value)),
			clientv3.OpPut(storageKeyMD, mdEncoded),
		)
	} else {
		mdBytes, err := json.Marshal(md)
		if err != nil {
			return errors.Wrap(err, "store: failed to marshal metadata")
		}
		mdEncoded := base64.StdEncoding.EncodeToString(mdBytes)

		txn.Then(
			clientv3.OpPut(storageKey, base64.StdEncoding.EncodeToString(value)),
			clientv3.OpPut(storageKeyMD, mdEncoded),
		)
	}

	txnResp, err := txn.Commit()
	if err != nil {
		return errors.Wrap(err, "store: failed to commit transaction")
	}
	if !txnResp.Succeeded {
		return errors.New("store: transaction failed")
	}

	return nil
}

func (e *etcdsrv) Load(key string) ([]byte, error) {
	storageKey := path.Join(e.cfg.KeyPrefix, key)
	storageKeyMD := path.Join(e.mdPrefix, key)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Increased timeout
	defer cancel()

	resp, err := e.cli.Get(ctx, storageKeyMD)
	if err != nil {
		return nil, errors.Wrap(err, "load: failed to get metadata")
	}
	if len(resp.Kvs) == 0 {
		return nil, NotExist{key}
	}

	md := new(Metadata)
	b, err := base64.StdEncoding.DecodeString(string(resp.Kvs[0].Value))
	if err != nil {
		return nil, errors.Wrap(err, "load: failed to decode metadata")
	}
	if err := json.Unmarshal(b, md); err != nil {
		return nil, errors.Wrap(err, "load: failed to unmarshal metadata")
	}

	valueResp, err := e.cli.Get(ctx, storageKey)
	if err != nil {
		return nil, errors.Wrap(err, "load: failed to get value")
	}
	if len(valueResp.Kvs) == 0 {
		return nil, errors.New("load: value not found but metadata exists")
	}

	value, err := base64.StdEncoding.DecodeString(string(valueResp.Kvs[0].Value))
	if err != nil {
		return nil, errors.Wrap(err, "load: failed to decode value")
	}

	if sha1.Sum(value) != md.Hash {
		return nil, FailedChecksum{key}
	}

	return value, nil
}

func (e *etcdsrv) Delete(key string) error {
	storageKey := path.Join(e.cfg.KeyPrefix, key)
	storageKeyMD := path.Join(e.mdPrefix, key)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	txn := e.cli.Txn(ctx)
	txn.Then(
		clientv3.OpDelete(storageKey),
		clientv3.OpDelete(storageKeyMD),
	)

	txnResp, err := txn.Commit()
	if err != nil {
		return errors.Wrap(err, "delete: failed to commit transaction")
	}
	if !txnResp.Succeeded {
		return errors.New("delete: transaction failed")
	}

	return nil
}

func (e *etcdsrv) Metadata(key string) (*Metadata, error) {
	storageKeyMD := path.Join(e.mdPrefix, key)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try direct file lookup first
	resp, err := e.cli.Get(ctx, storageKeyMD)
	if err != nil {
		return nil, errors.Wrap(err, "metadata: failed to get key")
	}

	if len(resp.Kvs) > 0 {
		md := new(Metadata)
		b, err := base64.StdEncoding.DecodeString(string(resp.Kvs[0].Value))
		if err != nil {
			return nil, errors.Wrap(err, "metadata: failed to decode base64")
		}
		if err := json.Unmarshal(b, md); err != nil {
			return nil, errors.Wrap(err, "metadata: failed to unmarshal")
		}
		md.Path = key
		return md, nil
	}

	// Look for children to determine if it's a directory
	dirResp, err := e.cli.Get(ctx, storageKeyMD+"/", clientv3.WithPrefix())
	if err != nil {
		return nil, errors.Wrap(err, "metadata: failed to get directory contents")
	}

	if len(dirResp.Kvs) == 0 {
		return nil, NotExist{key}
	}

	// It's a directory - aggregate metadata from children
	md := &Metadata{
		Path:  key,
		IsDir: true,
	}

	// Initialize timestamp to oldest possible time
	md.Timestamp = time.Time{}

	for _, kv := range dirResp.Kvs {
		childMd := new(Metadata)
		if err := unmarshalMDv3(kv.Value, childMd); err != nil {
			continue
		}
		md.Size += childMd.Size
		if childMd.Timestamp.After(md.Timestamp) {
			md.Timestamp = childMd.Timestamp
		}
	}

	// If no valid timestamps were found, use current time
	if md.Timestamp.IsZero() {
		md.Timestamp = time.Now().UTC()
	}

	return md, nil
}

func (e *etcdsrv) prefix() string {
	return e.cfg.KeyPrefix
}

// filter applies a filter function to a slice of KeyValue pairs
// filter applies a filter function to a slice of KeyValue pairs.
// It returns a new slice containing only the elements that pass the filter.
//
// Parameters:
//   - kvs: Slice of KeyValue pairs to filter
//   - f: Filter function that returns true for elements to keep
//
// Returns a new slice containing only the KeyValue pairs that passed the filter
func filter(kvs []*mvccpb.KeyValue, f func(*mvccpb.KeyValue) bool) []*mvccpb.KeyValue {
	var out []*mvccpb.KeyValue
	for _, kv := range kvs {
		if f(kv) {
			out = append(out, kv)
		}
	}
	return out
}

// FilterPrefix returns a filter function that matches keys with the given prefix
// FilterPrefix returns a filter function that matches keys with the given prefix.
// It trims the base prefix (cut) from each key before checking the match prefix.
//
// Parameters:
//   - prefix: The prefix to match against after trimming the base prefix
//   - cut: The base prefix to remove before matching (e.g. /caddy)
//
// Returns a filter function that can be used with List() to filter by key prefix
func FilterPrefix(prefix string, cut string) func(*mvccpb.KeyValue) bool {
	return func(kv *mvccpb.KeyValue) bool {
		return strings.HasPrefix(strings.TrimPrefix(string(kv.Key), cut), prefix)
	}
}

// FilterRemoveDirectories returns a filter function that removes directory entries
// A key is considered a directory if it has no value
// FilterRemoveDirectories returns a filter function that removes directory entries.
// A key is considered a directory if it has no value stored in etcd.
// This is useful for getting only leaf nodes (files) in a directory structure.
//
// Returns a filter function that can be used with List() to exclude directories
func FilterRemoveDirectories() func(*mvccpb.KeyValue) bool {
	return func(kv *mvccpb.KeyValue) bool {
		return len(kv.Value) > 0
	}
}

// FilterExactPrefix returns a filter function that matches only terminal nodes (files)
// with the exact path prefix
// FilterExactPrefix returns a filter function that matches only terminal nodes (files)
// with the exact path prefix. It trims the base prefix (cut) before matching.
//
// Parameters:
//   - prefix: The exact prefix to match after trimming (e.g. /path/to/dir)
//   - cut: The base prefix to remove before matching (e.g. /caddy)
//
// Returns a filter function that matches only files directly under the given prefix,
// not in subdirectories. For example, with prefix "/foo", it matches "/foo/file.txt"
// but not "/foo/bar/file.txt"
func FilterExactPrefix(prefix string, cut string) func(*mvccpb.KeyValue) bool {
	return func(kv *mvccpb.KeyValue) bool {
		s := strings.TrimPrefix(string(kv.Key), cut)
		if len(kv.Value) == 0 {
			return false // Directory
		}
		dir, _ := path.Split(s)
		if dir == prefix || dir == prefix+"/" {
			return true
		}
		return false
	}
}
