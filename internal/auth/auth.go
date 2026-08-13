// Package auth is F-17, slice 1: login, three profiles, permissions.
// The account management screen is cut from the slice.
//
// The contract the rest of the code relies on: an Identity in the request
// context, school and author deduced from it, never from a parameter
// (C-30). Handlers ask the Identity what the caller may do; the matrix
// itself is stable product structure, coded here on purpose — D-01 bans
// hard-coded BUSINESS values, not the shape of the product
// (MODELE_DONNEES §11).
package auth

import (
	"context"

	"github.com/google/uuid"
)

type Profile string

const (
	Management Profile = "MANAGEMENT"
	Office     Profile = "OFFICE"
	Instructor Profile = "INSTRUCTOR"
)

// Identity is the resolved caller. Rights are the UNION of open profiles
// (RG-208).
type Identity struct {
	UserID   uuid.UUID
	SchoolID uuid.UUID
	Name     string
	Profiles []Profile
}

// CanEditPlanning: office and management write the planner, instructors
// read it (F-17 matrix, slice 1).
func (id Identity) CanEditPlanning() bool {
	return id.has(Office) || id.has(Management)
}

// CanForceRelease: management only (RG-145).
func (id Identity) CanForceRelease() bool {
	return id.has(Management)
}

// CanManageResources / CanManagePeople: office and management, like the
// planner — the same people who plan enter the trucks and the students.
func (id Identity) CanManageResources() bool {
	return id.has(Office) || id.has(Management)
}

func (id Identity) CanManagePeople() bool {
	return id.has(Office) || id.has(Management)
}

// CanManageSettings: management only. Hypothesis of slice 1, flagged in
// the delivery notes — the matrix detail for F-01 was never written down.
func (id Identity) CanManageSettings() bool {
	return id.has(Management)
}

func (id Identity) has(p Profile) bool {
	for _, x := range id.Profiles {
		if x == p {
			return true
		}
	}
	return false
}

type ctxKey struct{}

func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

func withIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}
