package types

import (
	"fmt"
	"reflect"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12_381"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/crypto/sr25519"
)

//-------------------------------------------------------
// Use strings to distinguish types in ABCI messages

const (
	ABCIEvidenceTypeDuplicateVote    = "duplicate/vote"
	ABCIEvidenceTypeMock             = "mock/evidence"
	ABCIEvidenceTypeBeaconInactivity = "beacon/inactivity"
	ABCIEvidenceTypeDKG              = "beacon/dkg"
)

const (
	ABCIPubKeyTypeEd25519   = "ed25519"
	ABCIPubKeyTypeSr25519   = "sr25519"
	ABCIPubKeyTypeSecp256k1 = "secp256k1"
	ABCIPubKeyTypeBls12_381 = "bls12_381"
)

// TODO: Make non-global by allowing for registration of more pubkey types

var ABCIPubKeyTypesToAminoNames = map[string]string{
	ABCIPubKeyTypeEd25519:   ed25519.PubKeyAminoName,
	ABCIPubKeyTypeSr25519:   sr25519.PubKeyAminoName,
	ABCIPubKeyTypeSecp256k1: secp256k1.PubKeyAminoName,
	ABCIPubKeyTypeBls12_381: bls12_381.PubKeyAminoName,
}

//-------------------------------------------------------

// TM2PB is used for converting Tendermint ABCI to protobuf ABCI.
// UNSTABLE
var TM2PB = tm2pb{}

type tm2pb struct{}

func (tm2pb) Header(header *Header) abci.Header {
	return abci.Header{
		Version: abci.Version{
			Block: header.Version.Block.Uint64(),
			App:   header.Version.App.Uint64(),
		},
		ChainID: header.ChainID,
		Height:  header.Height,
		Time:    header.Time,

		LastBlockId: TM2PB.BlockID(header.LastBlockID),

		LastCommitHash: header.LastCommitHash,
		DataHash:       header.DataHash,

		ValidatorsHash:     header.ValidatorsHash,
		NextValidatorsHash: header.NextValidatorsHash,
		ConsensusHash:      header.ConsensusHash,
		AppHash:            header.AppHash,
		LastResultsHash:    header.LastResultsHash,

		EvidenceHash:    header.EvidenceHash,
		ProposerAddress: header.ProposerAddress,

		Entropy: TM2PB.BlockEntropy(header.Entropy),
	}
}

func (tm2pb) Validator(val *Validator) abci.Validator {
	return abci.Validator{
		Address: val.PubKey.Address(),
		Power:   val.VotingPower,
	}
}

func (tm2pb) BlockID(blockID BlockID) abci.BlockID {
	return abci.BlockID{
		Hash:        blockID.Hash,
		PartsHeader: TM2PB.PartSetHeader(blockID.PartsHeader),
	}
}

func (tm2pb) BlockEntropy(entropy BlockEntropy) abci.BlockEntropy {
	return abci.BlockEntropy{
		GroupSignature: entropy.GroupSignature,
		Round:          entropy.Round,
		AeonLength:     entropy.AeonLength,
		DkgId:          entropy.DKGID,
		NextAeonStart:  entropy.NextAeonStart,
		SuccessfulVals: entropy.Qual,
	}
}

func (tm2pb) PartSetHeader(header PartSetHeader) abci.PartSetHeader {
	return abci.PartSetHeader{
		Total: int32(header.Total),
		Hash:  header.Hash,
	}
}

// XXX: panics on unknown pubkey type
func (tm2pb) ValidatorUpdate(val *Validator) abci.ValidatorUpdate {
	return abci.ValidatorUpdate{
		PubKey: TM2PB.PubKey(val.PubKey),
		Power:  val.VotingPower,
	}
}

// XXX: panics on nil or unknown pubkey type
// TODO: add cases when new pubkey types are added to crypto
func (tm2pb) PubKey(pubKey crypto.PubKey) abci.PubKey {
	switch pk := pubKey.(type) {
	case ed25519.PubKeyEd25519:
		return abci.PubKey{
			Type: ABCIPubKeyTypeEd25519,
			Data: pk[:],
		}
	case sr25519.PubKeySr25519:
		return abci.PubKey{
			Type: ABCIPubKeyTypeSr25519,
			Data: pk[:],
		}
	case secp256k1.PubKeySecp256k1:
		return abci.PubKey{
			Type: ABCIPubKeyTypeSecp256k1,
			Data: pk[:],
		}
	case bls12_381.PubKeyBls:
		return abci.PubKey{
			Type: ABCIPubKeyTypeBls12_381,
			Data: pk[:],
		}
	default:
		panic(fmt.Sprintf("unknown pubkey type: %v %v", pubKey, reflect.TypeOf(pubKey)))
	}
}

// XXX: panics on nil or unknown pubkey type
func (tm2pb) ValidatorUpdates(vals *ValidatorSet) []abci.ValidatorUpdate {
	validators := make([]abci.ValidatorUpdate, vals.Size())
	for i, val := range vals.Validators {
		validators[i] = TM2PB.ValidatorUpdate(val)
	}
	return validators
}

func (tm2pb) ConsensusParams(params *ConsensusParams) *abci.ConsensusParams {
	return &abci.ConsensusParams{
		Block: &abci.BlockParams{
			MaxBytes: params.Block.MaxBytes,
			MaxGas:   params.Block.MaxGas,
		},
		Evidence: &abci.EvidenceParams{
			MaxAgeNumBlocks: params.Evidence.MaxAgeNumBlocks,
			MaxAgeDuration:  params.Evidence.MaxAgeDuration,
		},
		Validator: &abci.ValidatorParams{
			PubKeyTypes: params.Validator.PubKeyTypes,
		},
		Entropy: &abci.EntropyParams{
			AeonLength:                  params.Entropy.AeonLength,
			InactivityWindowSize:        params.Entropy.InactivityWindowSize,
			RequiredActivityPercentage:  params.Entropy.RequiredActivityPercentage,
			SlashingThresholdPercentage: params.Entropy.SlashingThresholdPercentage,
		},
	}
}

// ABCI Evidence includes information from the past that's not included in the evidence itself
// so Evidence types stays compact.
// XXX: panics on nil or unknown pubkey type
func (tm2pb) Evidence(ev Evidence, valSet *ValidatorSet, dkgValSet *ValidatorSet, evTime time.Time) abci.Evidence {

	evidence := abci.Evidence{
		Height:           ev.Height(),
		Time:             evTime,
		TotalVotingPower: valSet.TotalVotingPower(),
	}

	// set type and relevant validator set
	relevantValSet := valSet
	switch evType := ev.(type) {
	case *DuplicateVoteEvidence:
		evidence.Type = ABCIEvidenceTypeDuplicateVote
	case MockEvidence:
		// XXX: not great to have test types in production paths ...
		evidence.Type = ABCIEvidenceTypeMock
	case *BeaconInactivityEvidence:
		evidence.Type = ABCIEvidenceTypeBeaconInactivity
		if dkgValSet == nil {
			panic(fmt.Sprintf("TM2PB Evidence: received nil relevant val set: evType %v, height %v", evType, ev.ValidatorHeight()))
		}
		relevantValSet = dkgValSet
		evidence.Threshold = evType.Threshold
	case *DKGEvidence:
		evidence.Type = ABCIEvidenceTypeDKG
		if dkgValSet == nil {
			panic(fmt.Sprintf("TM2PB Evidence: received nil relevant val set: evType %v, height %v", evType, ev.ValidatorHeight()))
		}
		relevantValSet = dkgValSet
		evidence.Threshold = evType.Threshold
	default:
		panic(fmt.Sprintf("Unknown evidence type: %v %v", ev, reflect.TypeOf(ev)))
	}

	if relevantValSet == nil {
		panic(fmt.Sprintf("TM2PB Evidence: received nil relevant val set: evType %v, height %v", reflect.TypeOf(ev), ev.ValidatorHeight()))
	}
	_, val := relevantValSet.GetByAddress(ev.Address())
	if val == nil {
		panic(val)
	}
	evidence.Validator = TM2PB.Validator(val)

	return evidence
}

// XXX: panics on nil or unknown pubkey type
func (tm2pb) NewValidatorUpdate(pubkey crypto.PubKey, power int64) abci.ValidatorUpdate {
	pubkeyABCI := TM2PB.PubKey(pubkey)
	return abci.ValidatorUpdate{
		PubKey: pubkeyABCI,
		Power:  power,
	}
}

//----------------------------------------------------------------------------

// PB2TM is used for converting protobuf ABCI to Tendermint ABCI.
// UNSTABLE
var PB2TM = pb2tm{}

type pb2tm struct{}

func (pb2tm) PubKey(pubKey abci.PubKey) (crypto.PubKey, error) {
	switch pubKey.Type {
	case ABCIPubKeyTypeEd25519:
		if len(pubKey.Data) != ed25519.PubKeyEd25519Size {
			return nil, fmt.Errorf("invalid size for PubKeyEd25519. Got %d, expected %d",
				len(pubKey.Data), ed25519.PubKeyEd25519Size)
		}
		var pk ed25519.PubKeyEd25519
		copy(pk[:], pubKey.Data)
		return pk, nil
	case ABCIPubKeyTypeSr25519:
		if len(pubKey.Data) != sr25519.PubKeySr25519Size {
			return nil, fmt.Errorf("invalid size for PubKeySr25519. Got %d, expected %d",
				len(pubKey.Data), sr25519.PubKeySr25519Size)
		}
		var pk sr25519.PubKeySr25519
		copy(pk[:], pubKey.Data)
		return pk, nil
	case ABCIPubKeyTypeSecp256k1:
		if len(pubKey.Data) != secp256k1.PubKeySecp256k1Size {
			return nil, fmt.Errorf("invalid size for PubKeySecp256k1. Got %d, expected %d",
				len(pubKey.Data), secp256k1.PubKeySecp256k1Size)
		}
		var pk secp256k1.PubKeySecp256k1
		copy(pk[:], pubKey.Data)
		return pk, nil
	case ABCIPubKeyTypeBls12_381:
		if len(pubKey.Data) != bls12_381.TotalPubKeyBlsSize {
			return nil, fmt.Errorf("invalid size for PubKeyBls12_381. Got %d, expected %d",
				len(pubKey.Data), bls12_381.TotalPubKeyBlsSize)
		}
		var pk bls12_381.PubKeyBls
		copy(pk[:], pubKey.Data)
		return pk, nil
	default:
		return nil, fmt.Errorf("unknown pubkey type %v", pubKey.Type)
	}
}

func (pb2tm) ValidatorUpdates(vals []abci.ValidatorUpdate) ([]*Validator, error) {
	tmVals := make([]*Validator, len(vals))
	for i, v := range vals {
		pub, err := PB2TM.PubKey(v.PubKey)
		if err != nil {
			return nil, err
		}
		tmVals[i] = NewValidator(pub, v.Power)
	}
	return tmVals, nil
}
