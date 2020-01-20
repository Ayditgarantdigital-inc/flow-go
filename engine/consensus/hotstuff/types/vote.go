package types

import "encoding/binary"

type Vote struct {
	View      uint64
	BlockMRH  []byte
	Signature *Signature
}

func NewVote(view uint64, blockMRH []byte, sig *Signature) *Vote {
	return &Vote{
		View:      view,
		BlockMRH:  blockMRH,
		Signature: sig,
	}
}

func (uv *Vote) BytesForSig() []byte {
	return voteBytesForSig(uv.View, uv.BlockMRH)
}

func voteBytesForSig(view uint64, blockMRH []byte) []byte {
	// TODO there's probably a cleaner way to do this whole fcn
	voteStrBytes := []byte("vote")

	viewBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewBytes, view)

	length := len(voteStrBytes) + len(viewBytes) + len(blockMRH)
	bytesForSig := make([]byte, 0, length)
	bytesForSig = append(bytesForSig, voteStrBytes...)
	bytesForSig = append(bytesForSig, viewBytes...)
	bytesForSig = append(bytesForSig, blockMRH...)

	return bytesForSig
}
