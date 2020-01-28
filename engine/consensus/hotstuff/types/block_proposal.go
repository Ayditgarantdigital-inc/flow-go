package types

type BlockProposal struct {
	Block            *Block
	ConsensusPayload *ConsensusPayload
	Signature        *Signature // CAUTION: this is sign(Block), i.e. it does NOT include ConsensusPayload
}

func NewBlockProposal(block *Block, consensusPayload *ConsensusPayload, sig *Signature) *BlockProposal {
	return &BlockProposal{
		Block:            block,
		ConsensusPayload: consensusPayload,
		Signature:        sig,
	}
}

func (b *BlockProposal) QC() *QuorumCertificate { return b.Block.QC }
func (b *BlockProposal) View() uint64           { return b.Block.View }
func (b *BlockProposal) BlockMRH() []byte       { return b.Block.BlockMRH() }
func (b *BlockProposal) Height() uint64         { return b.Block.Height }
