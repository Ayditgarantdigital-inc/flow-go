// Code generated by capnpc-go. DO NOT EDIT.

package captain

import (
	capnp "zombiezen.com/go/capnproto2"
	text "zombiezen.com/go/capnproto2/encoding/text"
	schemas "zombiezen.com/go/capnproto2/schemas"
)

type Identity struct{ capnp.Struct }

// Identity_TypeID is the unique identifier for the type Identity.
const Identity_TypeID = 0x87bef45c1dc996c0

func NewIdentity(s *capnp.Segment) (Identity, error) {
	st, err := capnp.NewStruct(s, capnp.ObjectSize{DataSize: 16, PointerCount: 2})
	return Identity{st}, err
}

func NewRootIdentity(s *capnp.Segment) (Identity, error) {
	st, err := capnp.NewRootStruct(s, capnp.ObjectSize{DataSize: 16, PointerCount: 2})
	return Identity{st}, err
}

func ReadRootIdentity(msg *capnp.Message) (Identity, error) {
	root, err := msg.RootPtr()
	return Identity{root.Struct()}, err
}

func (s Identity) String() string {
	str, _ := text.Marshal(0x87bef45c1dc996c0, s.Struct)
	return str
}

func (s Identity) NodeId() ([]byte, error) {
	p, err := s.Struct.Ptr(0)
	return []byte(p.Data()), err
}

func (s Identity) HasNodeId() bool {
	p, err := s.Struct.Ptr(0)
	return p.IsValid() || err != nil
}

func (s Identity) SetNodeId(v []byte) error {
	return s.Struct.SetData(0, v)
}

func (s Identity) Address() (string, error) {
	p, err := s.Struct.Ptr(1)
	return p.Text(), err
}

func (s Identity) HasAddress() bool {
	p, err := s.Struct.Ptr(1)
	return p.IsValid() || err != nil
}

func (s Identity) AddressBytes() ([]byte, error) {
	p, err := s.Struct.Ptr(1)
	return p.TextBytes(), err
}

func (s Identity) SetAddress(v string) error {
	return s.Struct.SetText(1, v)
}

func (s Identity) Role() uint8 {
	return s.Struct.Uint8(0)
}

func (s Identity) SetRole(v uint8) {
	s.Struct.SetUint8(0, v)
}

func (s Identity) Stake() uint64 {
	return s.Struct.Uint64(8)
}

func (s Identity) SetStake(v uint64) {
	s.Struct.SetUint64(8, v)
}

// Identity_List is a list of Identity.
type Identity_List struct{ capnp.List }

// NewIdentity creates a new list of Identity.
func NewIdentity_List(s *capnp.Segment, sz int32) (Identity_List, error) {
	l, err := capnp.NewCompositeList(s, capnp.ObjectSize{DataSize: 16, PointerCount: 2}, sz)
	return Identity_List{l}, err
}

func (s Identity_List) At(i int) Identity { return Identity{s.List.Struct(i)} }

func (s Identity_List) Set(i int, v Identity) error { return s.List.SetStruct(i, v.Struct) }

func (s Identity_List) String() string {
	str, _ := text.MarshalList(0x87bef45c1dc996c0, s.List)
	return str
}

// Identity_Promise is a wrapper for a Identity promised by a client call.
type Identity_Promise struct{ *capnp.Pipeline }

func (p Identity_Promise) Struct() (Identity, error) {
	s, err := p.Pipeline.Struct()
	return Identity{s}, err
}

type Header struct{ capnp.Struct }

// Header_TypeID is the unique identifier for the type Header.
const Header_TypeID = 0xde11024ca833d34a

func NewHeader(s *capnp.Segment) (Header, error) {
	st, err := capnp.NewStruct(s, capnp.ObjectSize{DataSize: 16, PointerCount: 3})
	return Header{st}, err
}

func NewRootHeader(s *capnp.Segment) (Header, error) {
	st, err := capnp.NewRootStruct(s, capnp.ObjectSize{DataSize: 16, PointerCount: 3})
	return Header{st}, err
}

func ReadRootHeader(msg *capnp.Message) (Header, error) {
	root, err := msg.RootPtr()
	return Header{root.Struct()}, err
}

func (s Header) String() string {
	str, _ := text.Marshal(0xde11024ca833d34a, s.Struct)
	return str
}

func (s Header) Number() uint64 {
	return s.Struct.Uint64(0)
}

func (s Header) SetNumber(v uint64) {
	s.Struct.SetUint64(0, v)
}

func (s Header) Timestamp() uint64 {
	return s.Struct.Uint64(8)
}

func (s Header) SetTimestamp(v uint64) {
	s.Struct.SetUint64(8, v)
}

func (s Header) Parent() ([]byte, error) {
	p, err := s.Struct.Ptr(0)
	return []byte(p.Data()), err
}

func (s Header) HasParent() bool {
	p, err := s.Struct.Ptr(0)
	return p.IsValid() || err != nil
}

func (s Header) SetParent(v []byte) error {
	return s.Struct.SetData(0, v)
}

func (s Header) Payload() ([]byte, error) {
	p, err := s.Struct.Ptr(1)
	return []byte(p.Data()), err
}

func (s Header) HasPayload() bool {
	p, err := s.Struct.Ptr(1)
	return p.IsValid() || err != nil
}

func (s Header) SetPayload(v []byte) error {
	return s.Struct.SetData(1, v)
}

func (s Header) Signatures() (capnp.DataList, error) {
	p, err := s.Struct.Ptr(2)
	return capnp.DataList{List: p.List()}, err
}

func (s Header) HasSignatures() bool {
	p, err := s.Struct.Ptr(2)
	return p.IsValid() || err != nil
}

func (s Header) SetSignatures(v capnp.DataList) error {
	return s.Struct.SetPtr(2, v.List.ToPtr())
}

// NewSignatures sets the signatures field to a newly
// allocated capnp.DataList, preferring placement in s's segment.
func (s Header) NewSignatures(n int32) (capnp.DataList, error) {
	l, err := capnp.NewDataList(s.Struct.Segment(), n)
	if err != nil {
		return capnp.DataList{}, err
	}
	err = s.Struct.SetPtr(2, l.List.ToPtr())
	return l, err
}

// Header_List is a list of Header.
type Header_List struct{ capnp.List }

// NewHeader creates a new list of Header.
func NewHeader_List(s *capnp.Segment, sz int32) (Header_List, error) {
	l, err := capnp.NewCompositeList(s, capnp.ObjectSize{DataSize: 16, PointerCount: 3}, sz)
	return Header_List{l}, err
}

func (s Header_List) At(i int) Header { return Header{s.List.Struct(i)} }

func (s Header_List) Set(i int, v Header) error { return s.List.SetStruct(i, v.Struct) }

func (s Header_List) String() string {
	str, _ := text.MarshalList(0xde11024ca833d34a, s.List)
	return str
}

// Header_Promise is a wrapper for a Header promised by a client call.
type Header_Promise struct{ *capnp.Pipeline }

func (p Header_Promise) Struct() (Header, error) {
	s, err := p.Pipeline.Struct()
	return Header{s}, err
}

type Block struct{ capnp.Struct }

// Block_TypeID is the unique identifier for the type Block.
const Block_TypeID = 0xb21c60ff9d83fbf5

func NewBlock(s *capnp.Segment) (Block, error) {
	st, err := capnp.NewStruct(s, capnp.ObjectSize{DataSize: 0, PointerCount: 3})
	return Block{st}, err
}

func NewRootBlock(s *capnp.Segment) (Block, error) {
	st, err := capnp.NewRootStruct(s, capnp.ObjectSize{DataSize: 0, PointerCount: 3})
	return Block{st}, err
}

func ReadRootBlock(msg *capnp.Message) (Block, error) {
	root, err := msg.RootPtr()
	return Block{root.Struct()}, err
}

func (s Block) String() string {
	str, _ := text.Marshal(0xb21c60ff9d83fbf5, s.Struct)
	return str
}

func (s Block) Header() (Header, error) {
	p, err := s.Struct.Ptr(0)
	return Header{Struct: p.Struct()}, err
}

func (s Block) HasHeader() bool {
	p, err := s.Struct.Ptr(0)
	return p.IsValid() || err != nil
}

func (s Block) SetHeader(v Header) error {
	return s.Struct.SetPtr(0, v.Struct.ToPtr())
}

// NewHeader sets the header field to a newly
// allocated Header struct, preferring placement in s's segment.
func (s Block) NewHeader() (Header, error) {
	ss, err := NewHeader(s.Struct.Segment())
	if err != nil {
		return Header{}, err
	}
	err = s.Struct.SetPtr(0, ss.Struct.ToPtr())
	return ss, err
}

func (s Block) NewIdentities() (Identity_List, error) {
	p, err := s.Struct.Ptr(1)
	return Identity_List{List: p.List()}, err
}

func (s Block) HasNewIdentities() bool {
	p, err := s.Struct.Ptr(1)
	return p.IsValid() || err != nil
}

func (s Block) SetNewIdentities(v Identity_List) error {
	return s.Struct.SetPtr(1, v.List.ToPtr())
}

// NewNewIdentities sets the newIdentities field to a newly
// allocated Identity_List, preferring placement in s's segment.
func (s Block) NewNewIdentities(n int32) (Identity_List, error) {
	l, err := NewIdentity_List(s.Struct.Segment(), n)
	if err != nil {
		return Identity_List{}, err
	}
	err = s.Struct.SetPtr(1, l.List.ToPtr())
	return l, err
}

func (s Block) CollectionGuarantees() (CollectionGuarantee_List, error) {
	p, err := s.Struct.Ptr(2)
	return CollectionGuarantee_List{List: p.List()}, err
}

func (s Block) HasCollectionGuarantees() bool {
	p, err := s.Struct.Ptr(2)
	return p.IsValid() || err != nil
}

func (s Block) SetCollectionGuarantees(v CollectionGuarantee_List) error {
	return s.Struct.SetPtr(2, v.List.ToPtr())
}

// NewCollectionGuarantees sets the collectionGuarantees field to a newly
// allocated CollectionGuarantee_List, preferring placement in s's segment.
func (s Block) NewCollectionGuarantees(n int32) (CollectionGuarantee_List, error) {
	l, err := NewCollectionGuarantee_List(s.Struct.Segment(), n)
	if err != nil {
		return CollectionGuarantee_List{}, err
	}
	err = s.Struct.SetPtr(2, l.List.ToPtr())
	return l, err
}

// Block_List is a list of Block.
type Block_List struct{ capnp.List }

// NewBlock creates a new list of Block.
func NewBlock_List(s *capnp.Segment, sz int32) (Block_List, error) {
	l, err := capnp.NewCompositeList(s, capnp.ObjectSize{DataSize: 0, PointerCount: 3}, sz)
	return Block_List{l}, err
}

func (s Block_List) At(i int) Block { return Block{s.List.Struct(i)} }

func (s Block_List) Set(i int, v Block) error { return s.List.SetStruct(i, v.Struct) }

func (s Block_List) String() string {
	str, _ := text.MarshalList(0xb21c60ff9d83fbf5, s.List)
	return str
}

// Block_Promise is a wrapper for a Block promised by a client call.
type Block_Promise struct{ *capnp.Pipeline }

func (p Block_Promise) Struct() (Block, error) {
	s, err := p.Pipeline.Struct()
	return Block{s}, err
}

func (p Block_Promise) Header() Header_Promise {
	return Header_Promise{Pipeline: p.Pipeline.GetPipeline(0)}
}

const schema_cf3ad8085685b22d = "x\xda|\x91?k\x14]\x1c\x85\xcf\xb9w\x93}\xf3" +
	"\xb2I\xf6\x92m\x0c\xcaV)\x144\xc4ti\x12\x02" +
	"b\"\x11r\x05\x05%E\xae3Ww\xd8\xf9\xc7\xcc" +
	"]\xe2\x16\xc1\x88\x11\x15\x14\"\xa8\x95\xf6\"\x82\x90\x0f" +
	" 6\x16V\x166V\x96\xfa\x05\xb4Qpd\x96d" +
	"w\xb1H\xfb\x9b\xc3\xdc\xe7\x9c\xa7\xfegI\xcc\x8dL" +
	"\x12\xd0\xb5\x91\xd1\xe2\xfd\xf3\x8f'6~\xbc\xbb\x0f=" +
	"MQ\x9c\xde\xbfw\xe5\xbf/\x0b\x9f0\"\xaa\xc0\\" +
	"4M\xb5]\x05T\xf7;X\xfc\xfc}\xf7e\xb1y" +
	"|\x1fj\x9aCIY\x05\xe6/Rp\xea*\xab\xc0" +
	"\xd4e\xbe\x05\x8b\x0b\x9f\xe7_\xad\x09\xf5\xf5\xdf\xff\xf6" +
	"\xd2\xc7\xc4\xff\x9c:Y>1?#\x9a\x04\x8b\xdck" +
	"\xd9\xc8\xccz\xd2\xa4\xce\x04\xf1\xec\x8d0\xd9:\xe3\x99" +
	"4N\x17V}\x1b\xbb@\xba\xee:\xa9\xeb\xb2\x02T" +
	"\x08(\xb3\x00\xe8\x0dI\xdd\x12Td\x83\xe5\xd1.\x03" +
	"zSR\x87\x82\x14\x0d\x0a@\x05\xa7\x00\xedK\xeaT" +
	"PI6(\x01\x15\x9d\x05tKR;\xc1\xc58\xf1" +
	"\xed\xaa\xcfq\x08\x8e\x83\xb7\x8d\xefg6\xcfY\x83`" +
	"\x0d\x9c\xcc\x92\xd0r\x14\x82\xa3`3w\xa6m9\x06" +
	"\xc1\xb1\xa3\xb1\x97\xc3\xc4k\xa3d\xae\xf5\x99\xcf\x95\xcc" +
	"K\x92zm\x88y5\x03\xf4\x8a\xa4\xf6\x05\x958\x80" +
	"6\xaf\x0f\xa0w\x04\x17[\xd6\xf86c}0*\xc8" +
	":X\xc4v\xab\xb7\x0e\x9a\x81\x0bl\xce\x09p]\x92" +
	"\xf5\x81U\xb0<\x16^\x12\x86\xd6s\x01\x93\xf8|\xc7" +
	"d&\x9etv8?\xf3\xec\xdb\xafv\xf3\xc5\x87\xc3" +
	"\xfc\x11\xbdVz0\xe85k\xf4\x9bm\x97\xcdnI" +
	"\xea\xdd\xa1fw.\x01zGR?\x1a\xd8xX\x06" +
	"w%\xf5\xde\x90\x8d\xc7\xa5\xb6\x07\x92\xfa\xa9\xa0\xaa\x88" +
	"\x06+\x80zr\x0d\xd0{\x92\xfaM\xa9\xa8\x13]\xb7" +
	"Y\x7fw\x17D6w&\x02\xd3\xc3\xdbbj2\x1b" +
	"\xbb\xbe\xc5\xd4t\xc3\xc4\xf4\xad\x16yp36\xae\x93" +
	"A\x0e\x8a\x97\xdf&\xc0\xbf\x01\x00\x00\xff\xffVM\xb0" +
	"\xd3"

func init() {
	schemas.Register(schema_cf3ad8085685b22d,
		0x87bef45c1dc996c0,
		0xb21c60ff9d83fbf5,
		0xde11024ca833d34a)
}
