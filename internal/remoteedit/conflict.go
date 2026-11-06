package remoteedit

import "time"

const mtimeMargin = time.Second

type baseline struct {
	Hash    string
	Size    int64
	ModTime time.Time
}

type saveDecision int

const (
	saveUnchanged saveDecision = iota

	saveNeedContentHash

	saveConflict
)

func decideSave(base baseline, seen remoteObservation) (saveDecision, string) {
	if !seen.Exists {
		return saveConflict, ""
	}
	if base.Hash != "" && seen.Size == base.Size && withinMtimeMargin(seen.ModTime, base.ModTime) {
		return saveUnchanged, ""
	}
	if seen.Hash == "" {
		return saveNeedContentHash, ""
	}
	if seen.Hash == base.Hash {
		return saveUnchanged, ""
	}
	return saveConflict, seen.Hash
}

func withinMtimeMargin(left, right time.Time) bool {
	diff := left.Sub(right)
	if diff < 0 {
		diff = -diff
	}
	return diff < mtimeMargin
}

func decideExpectedWrite(expectedHash string, seen remoteObservation) (bool, string) {
	if !seen.Exists {
		return expectedHash == "", ""
	}
	if expectedHash == "" || expectedHash != seen.Hash {
		return false, seen.Hash
	}
	return true, seen.Hash
}
