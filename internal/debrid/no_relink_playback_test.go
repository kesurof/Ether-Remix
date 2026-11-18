package debrid

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"
)

func totalProviderCalls(r *engineRig) int {
	n := 0
	for _, p := range r.provs {
		n += p.totalCalls()
	}
	return n
}

// Issue #9: a media host that answers 502 over and over during a read never
// makes Ether ask the provider for a link or another provider; the read
// resumes on the same signed link once the host answers again.
func TestMediaHost502DoesNotAskProviderForLink(t *testing.T) {
	r := newEngineRig(t, 512<<10, -1)
	src, err := r.eng.Open(context.Background(), r.rel.ID, r.pl)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	tok := payloadToken(t, r.pl)
	before := totalProviderCalls(r)
	linkBefore := r.eng.Stats().LinkRefreshes

	r.media.mu.Lock()
	r.media.status[tok] = http.StatusBadGateway
	r.media.mu.Unlock()
	for i := 0; i < 3; i++ {
		if _, err := src.ReadAt(make([]byte, 10), int64(i)*testBlock); err == nil {
			t.Fatal("read succeeded while the media host answered 502")
		}
	}
	if got := totalProviderCalls(r); got != before {
		t.Fatalf("provider API called %d times during 502s, want 0", got-before)
	}
	r.media.mu.Lock()
	delete(r.media.status, tok)
	r.media.mu.Unlock()
	if got := readAll(t, src); !bytes.Equal(got, r.media.data) {
		t.Fatal("content after the host recovered")
	}
	if got := totalProviderCalls(r); got != before {
		t.Fatalf("provider API called %d times across the outage, want 0", got-before)
	}
	if r.eng.Stats().LinkRefreshes != linkBefore {
		t.Fatal("a link refresh was counted for a media host failure")
	}
}

// A link the host reports gone (410) is still renewed.
func TestGoneLinkIsStillRenewed(t *testing.T) {
	r := newEngineRig(t, 256<<10, -1)
	src, err := r.eng.Open(context.Background(), r.rel.ID, r.pl)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	tok := payloadToken(t, r.pl)
	r.media.mu.Lock()
	r.media.status[tok] = http.StatusGone
	r.media.mu.Unlock()
	links := r.provs["realdebrid-0"].callCount("link")
	if got := readAll(t, src); !bytes.Equal(got, r.media.data) {
		t.Fatal("content after renewal")
	}
	if r.provs["realdebrid-0"].callCount("link") <= links {
		t.Fatal("expired link was not renewed")
	}
}

// Issue #9: two dead-link refreshes of one file are at least relinkFloor
// apart, even when the host alternates refusals and good answers.
func TestDeadLinkRefreshIsPaced(t *testing.T) {
	now := time.Date(2026, 11, 18, 20, 0, 0, 0, time.UTC)
	f := &remoteFile{e: &Engine{now: func() time.Time { return now }}}
	if !f.relinkDue() {
		t.Fatal("first refresh refused")
	}
	now = now.Add(relinkFloor - time.Second)
	if f.relinkDue() {
		t.Fatal("second refresh allowed inside the floor")
	}
	now = now.Add(2 * time.Second)
	if !f.relinkDue() {
		t.Fatal("refresh refused after the floor")
	}
}
