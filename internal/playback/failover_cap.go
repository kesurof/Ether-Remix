package playback

import (
	"errors"
	"slices"

	"github.com/braedonsaunders/ether/internal/core"
)

// defaultFailoverAttempts is how many releases one playback tries when the
// owner sets nothing (resolver.failoverAttempts).
const defaultFailoverAttempts = 3

// failoverAttempts is the cap on the releases one open or failover tries:
// resolver.failoverAttempts (default 3), never above Options.MaxAttempts.
func (m *Manager) failoverAttempts() int {
	n := m.config().Resolver.FailoverAttempts
	if n <= 0 {
		n = defaultFailoverAttempts
	}
	return min(n, m.opts.MaxAttempts)
}

// accountError reports a failure of the whole debrid account (quota reached,
// key rejected): no other release that goes through a debrid service can
// play until the owner or the clock fixes it, so trying them is wasted work.
func accountError(err error) bool {
	return errors.Is(err, core.ErrDebridQuota) || errors.Is(err, core.ErrDebridAuth)
}

// dependsOnDebrid reports a release that plays only through a debrid
// account: a torrent, a verified AllDebrid file or an add-on link a debrid
// service unlocked. Usenet releases and plain HTTP links do not.
func dependsOnDebrid(rel core.Release) bool {
	return rel.Protocol == core.ProtocolTorrent || core.IsVerifiedFile(rel) || (rel.Protocol == core.ProtocolHTTP && rel.StreamDebrid != "")
}

// withoutDebrid drops the releases that depend on a debrid account.
func withoutDebrid(rels []core.Release) []core.Release {
	return slices.DeleteFunc(slices.Clone(rels), dependsOnDebrid)
}
