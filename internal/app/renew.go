package app

import (
	"context"
	"time"

	"github.com/pickrole/pickrole/internal/store"
)

// Automatic renewal (docs/adr/0023): while PickRole is open, the active
// profile is loaded again shortly before its credentials expire, so every
// terminal keeps working. Loading it also gets a fresh CodeArtifact token.
// The SSO session is renewed first, with its refresh token, when it is about
// to expire. Nothing here opens the browser: once the refresh token is gone,
// the user signs in again as before.
const (
	renewCheckEvery    = time.Minute
	renewCredsBefore   = 10 * time.Minute
	renewSessionBefore = 15 * time.Minute
)

// Events sent to the UI.
const (
	// EventOverview means the state changed in the background: reload it.
	EventOverview = "overview"
	// EventRenewError carries the message of a failed background renewal.
	EventRenewError = "renew-error"
)

func (s *Service) renewLoop(ctx context.Context) {
	t := time.NewTicker(renewCheckEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.renewAndNotify(now)
		}
	}
}

// renewAndNotify runs renewDue and tells the UI, reporting each distinct
// error once instead of every minute.
func (s *Service) renewAndNotify(now time.Time) {
	renewed, err := s.renewDue(now)
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	s.mu.Lock()
	changed := msg != s.renewErr
	s.renewErr = msg
	p := s.platform
	s.mu.Unlock()
	if p == nil {
		return
	}
	if msg != "" && changed {
		p.Emit(EventRenewError, msg)
	}
	if renewed || changed {
		p.Emit(EventOverview)
	}
}

// renewDue renews what expires soon at now and reports whether anything was
// renewed. It does nothing when the preference is off or when a renewal is
// already running.
func (s *Service) renewDue(now time.Time) (renewed bool, err error) {
	if !s.renewing.TryLock() {
		return false, nil
	}
	defer s.renewing.Unlock()

	s.mu.Lock()
	on := s.cfg.Preferences.AutoRenew
	client, ctx := s.client, s.ctx
	var active *store.Active
	if a := s.state.Active; a != nil {
		c := *a
		active = &c
	}
	s.mu.Unlock()
	if !on || client == nil {
		return false, nil
	}

	// The SSO session first: the profile needs it. A failure here is not
	// reported by itself; loading the profile below says what to do.
	if tok, _ := client.Session(); tok.RefreshToken != "" && tok.Expiry().Sub(now) < renewSessionBefore {
		if client.Refresh(ctx) == nil {
			renewed = true
		}
	}

	if active == nil || active.ExpiresAt.Sub(now) >= renewCredsBefore {
		return renewed, nil
	}
	if _, err := s.loadProfile(active.AccountID, active.Role, false); err != nil {
		return renewed, err
	}
	return true, nil
}
