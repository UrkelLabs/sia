package mdm

import (
	"encoding/binary"
	"fmt"

	"gitlab.com/NebulousLabs/Sia/crypto"
	"gitlab.com/NebulousLabs/Sia/modules"
	"gitlab.com/NebulousLabs/Sia/types"
	"gitlab.com/NebulousLabs/encoding"
)

// instructionSwapSector is an instruction that swaps two sectors of a file
// contract.
type instructionSwapSector struct {
	commonInstruction

	sector1Offset uint64
	sector2Offset uint64
}

// staticDecodeSwapSectorInstruction creates a new 'SwapSector' instruction from the
// provided generic instruction.
func (p *program) staticDecodeSwapSectorInstruction(instruction modules.Instruction) (instruction, error) {
	// Check specifier.
	if instruction.Specifier != modules.SpecifierSwapSector {
		return nil, fmt.Errorf("expected specifier %v but got %v",
			modules.SpecifierSwapSector, instruction.Specifier)
	}
	// Check args.
	if len(instruction.Args) != modules.RPCISwapSectorLen {
		return nil, fmt.Errorf("expected instruction to have len %v but was %v",
			modules.RPCISwapSectorLen, len(instruction.Args))
	}
	// Read args.
	sector1Offset := binary.LittleEndian.Uint64(instruction.Args[:8])
	sector2Offset := binary.LittleEndian.Uint64(instruction.Args[8:16])
	return &instructionSwapSector{
		commonInstruction: commonInstruction{
			staticData:        p.staticData,
			staticMerkleProof: instruction.Args[16] == 1,
			staticState:       p.staticProgramState,
		},
		sector1Offset: sector1Offset,
		sector2Offset: sector2Offset,
	}, nil
}

// Execute executes the 'SwapSector' instruction.
func (i *instructionSwapSector) Execute(prevOutput output) output {
	// Fetch the data.
	offset1, err := i.staticData.Uint64(i.sector1Offset)
	if err != nil {
		return errOutput(err)
	}
	offset2, err := i.staticData.Uint64(i.sector2Offset)
	if err != nil {
		return errOutput(err)
	}

	// Order the offsets so we don't need to do that later.
	if offset2 < offset1 {
		offset1, offset2 = offset2, offset1
	}

	ps := i.staticState
	newMerkleRoot, err := ps.sectors.swapSectors(offset1, offset2)
	if err != nil {
		return errOutput(err)
	}

	// Get the swapped sectors. Since they have been swapped, the indices are
	// reversed.
	newRoots := i.staticState.sectors.merkleRoots
	oldSector1 := newRoots[offset2]
	oldSector2 := newRoots[offset1]

	// Construct proof if necessary.
	var proof []crypto.Hash
	var data []byte
	if i.staticMerkleProof {
		// Create the first range.
		var ranges []crypto.ProofRange
		var oldLeafHashes []crypto.Hash
		ranges = append(ranges, crypto.ProofRange{
			Start: offset1,
			End:   offset1 + 1,
		})
		oldLeafHashes = append(oldLeafHashes, oldSector1)
		// We only need the second range if the offsets aren't equal.
		if offset1 != offset2 {
			ranges = append(ranges, crypto.ProofRange{
				Start: offset2,
				End:   offset2 + 1,
			})
			oldLeafHashes = append(oldLeafHashes, oldSector2)
		}
		proof = crypto.MerkleDiffProof(ranges, uint64(len(newRoots)), nil, ps.sectors.merkleRoots)
		data = encoding.Marshal(oldLeafHashes)
	}

	return output{
		NewSize:       prevOutput.NewSize,
		NewMerkleRoot: newMerkleRoot,
		Output:        data,
		Proof:         proof,
	}
}

// Collateral returns the collateral cost of adding one full sector.
func (i *instructionSwapSector) Collateral() types.Currency {
	return modules.MDMSwapSectorCollateral()
}

// Cost returns the Cost of this `SwapSector` instruction.
func (i *instructionSwapSector) Cost() (executionCost, storage types.Currency, err error) {
	executionCost = modules.MDMSwapSectorCost(i.staticState.priceTable)
	return
}

// Memory returns the memory allocated by the 'SwapSector' instruction beyond the
// lifetime of the instruction.
func (i *instructionSwapSector) Memory() uint64 {
	return modules.MDMSwapSectorMemory()
}

// Time returns the execution time of an 'SwapSector' instruction.
func (i *instructionSwapSector) Time() (uint64, error) {
	return modules.MDMTimeSwapSector, nil
}
