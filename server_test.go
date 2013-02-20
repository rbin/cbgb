package cbgb

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/dustin/gomemcached"
)

func TestSaslListMechs(t *testing.T) {
	rh := reqHandler{currentBucket: nil}
	res := rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_LIST_MECHS,
	})
	if res == nil {
		t.Errorf("expected SASL_LIST_MECHS to be non-nil")
	}
	if !bytes.Equal(res.Body, []byte("PLAIN")) {
		t.Errorf("expected SASL_LIST_MECHS to be PLAIN")
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode:  gomemcached.SASL_LIST_MECHS,
		VBucket: 1,
	})
	if res.Status != gomemcached.EINVAL {
		t.Errorf("expected SASL_LIST_MECHS to fail, got: %v", res)
	}
}

func TestSaslBadAuthReq(t *testing.T) {
	rh := reqHandler{currentBucket: nil}
	res := rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
	})
	if res.Status != gomemcached.EINVAL {
		t.Errorf("expected SASL_AUTH with bad mech/key to fail, got: %v", res)
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode:  gomemcached.SASL_AUTH,
		VBucket: 1,
		Key:     []byte("PLAIN"),
	})
	if res.Status != gomemcached.EINVAL {
		t.Errorf("expected SASL_AUTH with nonzero vbucket to fail, got: %v", res)
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("\x00"),
	})
	if res.Status != gomemcached.EINVAL {
		t.Errorf("expected SASL_AUTH with short body to fail, got: %v", res)
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("aaa"),
	})
	if res.Status != gomemcached.EINVAL {
		t.Errorf("expected SASL_AUTH with bad body to fail, got: %v", res)
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("\x00aa"),
	})
	if res.Status != gomemcached.EINVAL {
		t.Errorf("expected SASL_AUTH with bad body to fail, got: %v", res)
	}
}

func TestSaslRejectedAuth(t *testing.T) {
	testBucketDir, _ := ioutil.TempDir("./tmp", "test")
	defer os.RemoveAll(testBucketDir)
	buckets, err := NewBuckets(testBucketDir,
		&BucketSettings{
			FlushInterval:   10 * time.Second,
			SleepInterval:   10 * time.Second,
			CompactInterval: 10 * time.Second,
		})
	if err != nil {
		t.Fatalf("Expected NewBuckets to succeed: %v", err)
	}
	rh := reqHandler{currentBucket: nil, buckets: buckets}
	res := rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("\x00not-a-bucket\x00"),
	})
	if res.Status != gomemcached.EINVAL {
		t.Errorf("expected SASL_AUTH to fail, got: %v", res)
	}
	if rh.currentBucket != nil {
		t.Errorf("expected currentBucket to be nil")
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("\x00not-a-bucket\x00some-pswd"),
	})
	if res.Status != gomemcached.EINVAL {
		t.Errorf("expected SASL_AUTH to fail, got: %v", res)
	}
	if rh.currentBucket != nil {
		t.Errorf("expected currentBucket to be nil")
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("\x00\x00some-pswd-but-missing-a-bucket"),
	})
	if res.Status != gomemcached.EINVAL {
		t.Errorf("expected SASL_AUTH to fail, got: %v", res)
	}
	if rh.currentBucket != nil {
		t.Errorf("expected currentBucket to be nil")
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("\x00\x00"),
	})
	if res.Status != gomemcached.EINVAL {
		t.Errorf("expected SASL_AUTH to fail, got: %v", res)
	}
	if rh.currentBucket != nil {
		t.Errorf("expected currentBucket to be nil")
	}

}

func TestSaslAuth(t *testing.T) {
	testBucketDir, _ := ioutil.TempDir("./tmp", "test")
	defer os.RemoveAll(testBucketDir)
	buckets, err := NewBuckets(testBucketDir,
		&BucketSettings{
			FlushInterval:   10 * time.Second,
			SleepInterval:   10 * time.Second,
			CompactInterval: 10 * time.Second,
		})
	if err != nil {
		t.Fatalf("Expected NewBuckets to succeed: %v", err)
	}
	nopwd, err := buckets.New("nopwd",
		&BucketSettings{
			FlushInterval:   10 * time.Second,
			SleepInterval:   10 * time.Second,
			CompactInterval: 10 * time.Second,
		})
	haspwd, err := buckets.New("haspwd",
		&BucketSettings{
			PasswordHash:    "a nice password",
			FlushInterval:   10 * time.Second,
			SleepInterval:   10 * time.Second,
			CompactInterval: 10 * time.Second,
		})
	rh := reqHandler{currentBucket: nil, buckets: buckets}
	res := rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("\x00nopwd\x00"),
	})
	if res.Status != gomemcached.SUCCESS {
		t.Errorf("expected SASL_AUTH to succeed, got: %v", res)
	}
	if rh.currentBucket != nopwd {
		t.Errorf("expected currentBucket to be nil")
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("\x00nopwd\x00wrong pswd"),
	})
	if res.Status != gomemcached.EINVAL {
		t.Errorf("expected SASL_AUTH to fail, got: %v", res)
	}
	if rh.currentBucket != nopwd {
		t.Errorf("expected currentBucket to be nil")
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("\x00haspwd\x00a nice password"),
	})
	if res.Status != gomemcached.SUCCESS {
		t.Errorf("expected SASL_AUTH to succeed, got: %v", res)
	}
	if rh.currentBucket != haspwd {
		t.Errorf("expected currentBucket to be nil")
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("\x00haspwd\x00a badpassword"),
	})
	if res.Status == gomemcached.SUCCESS {
		t.Errorf("expected SASL_AUTH to fail, got: %v", res)
	}
	if rh.currentBucket != haspwd {
		t.Errorf("expected currentBucket to be nil")
	}
	res = rh.HandleMessage(ioutil.Discard, &gomemcached.MCRequest{
		Opcode: gomemcached.SASL_AUTH,
		Key:    []byte("PLAIN"),
		Body:   []byte("\x00haspwd\x00"),
	})
	if res.Status == gomemcached.SUCCESS {
		t.Errorf("expected SASL_AUTH to fail, got: %v", res)
	}
	if rh.currentBucket != haspwd {
		t.Errorf("expected currentBucket to be nil")
	}
}
