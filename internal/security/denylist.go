package security

import (
    "sync"
    "github.com/google/uuid"
)

// In-memory deny lists; consider persistence for production.
var (
    muDenied sync.RWMutex
    denyUsers = make(map[uuid.UUID]struct{})
    denyIdent = make(map[string]struct{}) // key: provider + "|" + subject
)

func keyIdent(provider, subject string) string { return provider + "|" + subject }

// User denylist API
func DenyUser(id uuid.UUID)          { muDenied.Lock(); denyUsers[id] = struct{}{}; muDenied.Unlock() }
func AllowUser(id uuid.UUID)         { muDenied.Lock(); delete(denyUsers, id); muDenied.Unlock() }
func IsUserDenied(id uuid.UUID) bool { muDenied.RLock(); _, ok := denyUsers[id]; muDenied.RUnlock(); return ok }

// Identity denylist API (provider + subject)
func DenyIdentity(provider, subject string)   { muDenied.Lock(); denyIdent[keyIdent(provider, subject)] = struct{}{}; muDenied.Unlock() }
func AllowIdentity(provider, subject string)  { muDenied.Lock(); delete(denyIdent, keyIdent(provider, subject)); muDenied.Unlock() }
func IsIdentityDenied(provider, subject string) bool {
    muDenied.RLock(); _, ok := denyIdent[keyIdent(provider, subject)]; muDenied.RUnlock(); return ok
}

