package mdm

import (
	"errors"
	"fmt"

	"gitlab.com/NebulousLabs/Sia/crypto"
	"gitlab.com/NebulousLabs/Sia/modules"
)

// sectors contains the program cache, including gained and removed sectors as
// well as the list of sector roots.
type sectors struct {
	sectorsRemoved map[crypto.Hash]struct{}
	sectorsGained  map[crypto.Hash][]byte
	merkleRoots    []crypto.Hash
}

// newSectors creates a program cache given an initial list of sector roots.
func newSectors(roots []crypto.Hash) sectors {
	return sectors{
		sectorsRemoved: make(map[crypto.Hash]struct{}),
		sectorsGained:  make(map[crypto.Hash][]byte),
		merkleRoots:    roots,
	}
}

// appendSector adds the data to the program cache and returns the new merkle
// root.
func (s *sectors) appendSector(sectorData []byte) (crypto.Hash, error) {
	if uint64(len(sectorData)) != modules.SectorSize {
		return crypto.Hash{}, fmt.Errorf("trying to append data of length %v", len(sectorData))
	}
	newRoot := crypto.MerkleRoot(sectorData)

	// Add the sector to the cache. If it has been marked as removed, unmark it.
	s.sectorsGained[newRoot] = sectorData
	if _, prs := s.sectorsRemoved[newRoot]; prs {
		delete(s.sectorsRemoved, newRoot)
	}

	// Update the roots.
	s.merkleRoots = append(s.merkleRoots, newRoot)

	// Return the new merkle root of the contract.
	return cachedMerkleRoot(s.merkleRoots), nil
}

// dropSectors drops the specified number of sectors and returns the new merkle
// root.
func (s *sectors) dropSectors(numSectorsDropped uint64) (crypto.Hash, error) {
	oldNumSectors := uint64(len(s.merkleRoots))
	if numSectorsDropped > oldNumSectors {
		return crypto.Hash{}, fmt.Errorf("trying to drop %v sectors which is more than the amount of sectors (%v)", numSectorsDropped, oldNumSectors)
	}
	newNumSectors := oldNumSectors - numSectorsDropped

	// Update the roots.
	droppedRoots := s.merkleRoots[newNumSectors:]
	s.merkleRoots = s.merkleRoots[:newNumSectors]

	// Update the program cache.
	for _, droppedRoot := range droppedRoots {
		_, prs := s.sectorsGained[droppedRoot]
		if prs {
			// Remove the sectors from the cache.
			delete(s.sectorsGained, droppedRoot)
		} else {
			// Mark the sectors as removed in the cache.
			s.sectorsRemoved[droppedRoot] = struct{}{}
		}
	}

	// Compute the new merkle root of the contract.
	return cachedMerkleRoot(s.merkleRoots), nil
}

// hasSector checks if the given root exists, first checking the program cache
// and then querying the host.
func (s *sectors) hasSector(sectorRoot crypto.Hash) bool {
	for _, root := range s.merkleRoots {
		if root == sectorRoot {
			return true
		}
	}
	return false
}

// readSector reads data from the given root, returning the entire sector.
func (s *sectors) readSector(host Host, sectorRoot crypto.Hash) ([]byte, error) {
	// Check if the sector exists first-- otherwise the root wasn't added, or
	// was deleted.
	if !s.hasSector(sectorRoot) {
		return nil, errors.New("root not found in list of roots")
	}

	// The root exists. First check the gained sectors.
	if data, exists := s.sectorsGained[sectorRoot]; exists {
		return data, nil
	}

	// Check the host.
	return host.ReadSector(sectorRoot)
}
