package edition

import "fmt"

// Require refuses what only the Enterprise image can do, naming the act rather
// than a feature key: there are no keys any more, and "arranging the built-in
// pages is part of the Enterprise edition" is a sentence an operator can act
// on, where "white-label is not licensed" was a word from a price list.
func Require(what string) error {
	if Enterprise {
		return nil
	}
	return fmt.Errorf("%s is part of the Enterprise edition, and this is the community image", what)
}
