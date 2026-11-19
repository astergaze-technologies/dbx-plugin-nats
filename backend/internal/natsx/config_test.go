package natsx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nkeys"
)

func TestAuthModes(t *testing.T) {
	cfg := runServer(t, func(o *server.Options) { o.Username, o.Password = "admin", "s3cret" })
	if _, err := Test(cfg); err == nil {
		t.Fatal("expected authorization failure without credentials")
	}
	bad := cfg
	bad.Auth, bad.Username, bad.Password = AuthPassword, "admin", "wrong"
	if _, err := Test(bad); err == nil || strings.Contains(err.Error(), "wrong") {
		t.Fatalf("expected failure without leaking the password, got %v", err)
	}
	good := cfg
	good.Auth, good.Username, good.Password = AuthPassword, "admin", "s3cret"
	if msg, err := Test(good); err != nil || !strings.Contains(msg, "Connected to") {
		t.Fatalf("Test = %q, %v", msg, err)
	}

	token := runServer(t, func(o *server.Options) { o.Authorization = "tok" })
	token.Auth, token.Token = AuthToken, "tok"
	if _, err := Test(token); err != nil {
		t.Fatalf("token auth: %v", err)
	}
	if _, err := (Config{Auth: AuthToken}).options(); err == nil {
		t.Fatal("token auth without a token must fail")
	}
	if _, err := (Config{Auth: AuthCreds}).options(); err == nil {
		t.Fatal("creds auth without a file must fail")
	}

}

func TestNKeyAuth(t *testing.T) {
	user, _ := nkeys.CreateUser()
	pub, _ := user.PublicKey()
	seedBytes, _ := user.Seed()
	seed := string(seedBytes)
	srv := runServer(t, func(o *server.Options) { o.Nkeys = []*server.NkeyUser{{Nkey: pub}} })

	dir := t.TempDir()
	writeFile := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	stranger, _ := nkeys.CreateUser()
	strangerSeed, _ := stranger.Seed()
	acct, _ := nkeys.CreateAccount()
	acctSeed, _ := acct.Seed()
	decorated := "-----BEGIN USER NKEY SEED-----\n" + seed + "\n------END USER NKEY SEED------\n"

	ok := []struct {
		name       string
		seed, path string
	}{
		{"pasted seed", seed, ""},
		{"pasted seed with whitespace", "  " + seed + "\n", ""},
		{"decorated seed", decorated, ""},
		{"seed file", "", writeFile("user.nk", seed+"\n")},
		{"pasted wins over file", seed, filepath.Join(dir, "missing.nk")},
	}
	for _, c := range ok {
		t.Run(c.name, func(t *testing.T) {
			cfg := srv
			cfg.Auth, cfg.NKeySeed, cfg.NKeyPath = AuthNKey, c.seed, c.path
			if _, err := Test(cfg); err != nil {
				t.Fatalf("nkey auth: %v", err)
			}
		})
	}

	fail := []struct {
		name       string
		seed, path string
		online     bool // reaches the server rather than failing locally
	}{
		{"no seed", "", "", false},
		{"whitespace-only seed", " \n", "", false},
		{"empty file", "", writeFile("empty.nk", "\n"), false},
		{"missing file", "", filepath.Join(dir, "missing.nk"), false},
		{"garbage", "not-a-seed", "", false},
		{"account seed", string(acctSeed), "", false},
		{"public key instead of seed", pub, "", false},
		{"unknown user", string(strangerSeed), "", true},
	}
	for _, c := range fail {
		t.Run(c.name, func(t *testing.T) {
			cfg := srv
			cfg.Auth, cfg.NKeySeed, cfg.NKeyPath = AuthNKey, c.seed, c.path
			_, optErr := cfg.options()
			if (optErr == nil) != c.online {
				t.Fatalf("options() err = %v, want local failure=%v", optErr, !c.online)
			}
			_, err := Test(cfg)
			if err == nil {
				t.Fatal("expected failure")
			}
			if c.seed != "" && strings.Contains(err.Error(), c.seed) {
				t.Fatalf("error leaks the seed: %v", err)
			}
		})
	}
}

func TestOverviewAndNoJetStream(t *testing.T) {
	cfg := runServer(t, nil)
	seed(t, cfg)
	m, id := connect(t, cfg)
	ov, err := m.Overview(context.Background(), id)
	if err != nil || ov.Server.Version == "" || ov.JetStream == nil || ov.JetStream.Streams != 2 {
		t.Fatalf("overview = %+v, %v", ov, err) // ORDERS + KV_config
	}

	plain := runServer(t, func(o *server.Options) { o.JetStream = false })
	m2, id2 := connect(t, plain)
	ctx := context.Background()
	if ov, err := m2.Overview(ctx, id2); err != nil || ov.JetStream != nil {
		t.Fatalf("overview without JetStream = %+v, %v", ov, err)
	}
	for name, list := range map[string]func() (int, error){
		"streams": func() (int, error) { s, err := m2.Streams(ctx, id2); return len(s), err },
		"buckets": func() (int, error) { b, err := m2.Buckets(ctx, id2); return len(b), err },
		"objects": func() (int, error) { o, err := m2.ObjectStores(ctx, id2); return len(o), err },
	} {
		if n, err := list(); err != nil || n != 0 {
			t.Errorf("%s without JetStream = %d, %v", name, n, err)
		}
	}
}

func TestReadOnlyRejectsWrites(t *testing.T) {
	cfg := runServer(t, nil)
	seed(t, cfg)
	cfg.ReadOnly = true
	m, id := connect(t, cfg)
	ctx := context.Background()
	writes := map[string]error{
		"delete stream": m.DeleteStream(ctx, id, "ORDERS"),
		"purge stream":  m.PurgeStream(ctx, id, "ORDERS", ""),
		"delete bucket": m.DeleteBucket(ctx, id, "config"),
		"delete key":    m.DeleteKey(ctx, id, "config", "a", false),
		"create store":  m.CreateObjectStore(ctx, id, "files", ""),
	}
	_, putErr := m.Put(ctx, id, "config", "a", []byte("x"))
	writes["put key"] = putErr
	for name, err := range writes {
		if !errors.Is(err, ErrReadOnly) {
			t.Errorf("%s: got %v, want ErrReadOnly", name, err)
		}
	}
	if _, err := m.Streams(ctx, id); err != nil {
		t.Fatalf("reads must still work: %v", err)
	}
}

func TestValidateSubject(t *testing.T) {
	cases := []struct {
		subject  string
		wildcard bool
		ok       bool
	}{
		{"orders.created", false, true},
		{"orders.*", true, true},
		{"orders.>", true, true},
		{"orders.>", false, false},
		{">.orders", true, false},
		{"orders..x", true, false},
		{"", true, false},
		{"has space", true, false},
	}
	for _, c := range cases {
		if err := ValidateSubject(c.subject, c.wildcard); (err == nil) != c.ok {
			t.Errorf("ValidateSubject(%q, %v) = %v, want ok=%v", c.subject, c.wildcard, err, c.ok)
		}
	}
}
