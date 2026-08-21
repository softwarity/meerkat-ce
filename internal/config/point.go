package config

import (
	"context"
	"errors"
	"time"

	"github.com/softwarity/meerkat/internal/store"
)

// Record takes a restore point if the configuration has MOVED since the last
// one, and reports whether it wrote anything (CFG-06).
//
// It lives here rather than in the admin API because two callers need exactly
// the same answer, and two implementations of "has it moved" would be two
// answers waiting to disagree: every admin write asks it, and so does the
// gateway at startup - a tape whose first point is the first change can never
// go back to how things were before anyone touched them.
//
// The document is stored WITHOUT its pictures, which is what makes a tape
// affordable: a few kilobytes a point rather than a megabyte of base64 kept two
// hundred times over.
func Record(ctx context.Context, st *store.Store, actorID, label string) (bool, error) {
	doc, _, err := Export(ctx, st)
	if err != nil {
		return false, err
	}
	stripped, err := WithoutImages(doc)
	if err != nil {
		return false, err
	}
	file, err := Marshal(stripped)
	if err != nil {
		return false, err
	}
	digest := store.DigestOf(string(file))
	switch last, err := st.LastConfigPoint(ctx); {
	case err == nil && last.Digest == digest:
		return false, nil // nothing about the configuration moved
	case err != nil && !errors.Is(err, store.ErrNoConfigPoint):
		return false, err
	}
	if err := st.AddConfigPoint(ctx, store.ConfigPoint{
		At: time.Now().Unix(), ActorID: actorID, Label: label,
		Digest: digest, Document: string(file),
	}); err != nil {
		return false, err
	}
	return true, nil
}
