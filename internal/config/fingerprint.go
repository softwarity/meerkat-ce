package config

import (
	"github.com/softwarity/meerkat/internal/store"
)

// Fingerprint is the digest of a document as TEXT: pictures out, every object
// the way an export writes it. What a restore point stores and is keyed by
// (CFG-06), and half of what "this saved configuration is what runs" means
// (see Matches).
//
// One function, because the answer used to be computed four ways - the
// console's capture kept the pictures, the agent's listing took them out, the
// tape took them out, a pulled file kept them - and a gateway with a logo then
// said "not saved" right after a capture: the two fingerprints were of two
// different renderings of the same state.
func Fingerprint(doc *Document) (string, error) {
	file, err := Text(doc)
	if err != nil {
		return "", err
	}
	return store.DigestOf(string(file)), nil
}

// Text renders a document the way it is read and fingerprinted: pictures out,
// every object as an export writes it. A restore point stores exactly this.
func Text(doc *Document) ([]byte, error) {
	stripped, err := WithoutImages(exported(doc))
	if err != nil {
		return nil, err
	}
	return Marshal(stripped)
}

// Matches reports whether a saved configuration IS what runs: whether
// activating it would change nothing.
//
// That is the honest form of "saved", and it is why the pictures are judged
// apart from the text. A document without them - an export, a restore point,
// an agent's copy, a file edited in the console - keeps the ones in place when
// it is applied (importSettings), so their absence is no difference; a picture
// it DOES carry is one it would put up, so it has to be the one shown now.
func Matches(saved, running *Document) (bool, error) {
	a, err := Fingerprint(saved)
	if err != nil {
		return false, err
	}
	b, err := Fingerprint(running)
	if err != nil {
		return false, err
	}
	if a != b {
		return false, nil
	}
	mine, err := brandingMap(saved)
	if err != nil {
		return false, err
	}
	theirs, err := brandingMap(running)
	if err != nil {
		return false, err
	}
	for _, f := range imageFields {
		picture, _ := getPath(mine, f.path).(string)
		if picture == "" {
			continue // carried by nothing: the one in place stays
		}
		if shown, _ := getPath(theirs, f.path).(string); shown != picture {
			return false, nil
		}
	}
	return true, nil
}
