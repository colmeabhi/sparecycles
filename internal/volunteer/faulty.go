package volunteer

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
)

// maybeCorrupt returns a wrong output for a FaultyRate share of units, and the
// real output for the others. It lets a demo or a test run a faulty volunteer
// without a separate program.
func (v *Volunteer) maybeCorrupt(unitID string, output []byte) []byte {
	// rand.Float64 is in [0, 1), so a rate of 1 corrupts every unit and 0 none.
	if rand.Float64() >= v.cfg.FaultyRate {
		return output
	}
	v.log.Warn("faulty mode: sent wrong output", "unit", unitID)
	return corrupt(output)
}

// corrupt returns a copy of output that is always different from it.
//
// The change is random, so two faulty volunteers almost never send the same
// wrong output and cannot outvote an honest one by chance. A fixed change, such
// as flipping one bit, would make them agree. Faulty volunteers that agree on
// purpose (collusion) are a different attack, which voting alone does not stop.
func corrupt(output []byte) []byte {
	if len(output) == 0 {
		return []byte{byte(1 + rand.IntN(255))}
	}
	out := bytes.Clone(output)
	var mask [8]byte
	// The low bit is set, so the first byte always changes.
	binary.LittleEndian.PutUint64(mask[:], rand.Uint64()|1)
	for i := 0; i < len(out) && i < len(mask); i++ {
		out[i] ^= mask[i]
	}
	return out
}
