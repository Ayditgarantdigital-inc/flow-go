package network

import (
	"encoding/hex"
	"fmt"
	"strings"

	sdk "github.com/dapperlabs/flow-go-sdk"
)

type CadenceCode interface {
	ToCadence() string
}

type Transaction struct {
	Import  Import
	Content CadenceCode
}

func (t Transaction) ToCadence() string {
	return fmt.Sprintf("%s \n transaction { %s }", t.Import.ToCadence(), t.Content.ToCadence())

}

type Prepare struct {
	Content CadenceCode
}

func (t Prepare) ToCadence() string {
	return fmt.Sprintf("prepare(signer: Account) { %s }", t.Content.ToCadence())
}

type Execute struct {
	Content CadenceCode
}

func (t Execute) ToCadence() string {
	return fmt.Sprintf("execute { %s }", t.Content.ToCadence())
}

type Contract struct {
	Name    string
	Members []CadenceCode
}

func (t Contract) ToCadence() string {

	memberStrings := make([]string, len(t.Members))
	for i, member := range t.Members {
		memberStrings[i] = member.ToCadence()
	}

	return fmt.Sprintf("pub contract %s { %s }", t.Name, strings.Join(memberStrings, "\n"))
}

type Resource struct {
	Name string
	Code string
}

func (t Resource) ToCadence() string {
	return fmt.Sprintf("pub resource %s { %s }", t.Name, t.Code)
}

type Import struct {
	Address sdk.Address
}

func (t Import) ToCadence() string {
	if t.Address != sdk.ZeroAddress {
		return fmt.Sprintf("import 0x%s", t.Address.Short())
	}
	return ""
}

type UpdateAccountCode struct {
	Code string
}

func (t UpdateAccountCode) ToCadence() string {

	bytes := []byte(t.Code)

	hexCode := hex.EncodeToString(bytes)

	return fmt.Sprintf(`
		let code = "%s"

		fun hexDecode(_ s: String): [Int] {

				if s.length %% 2 != 0 {
					panic("Input must have even number of characters")
				}

				let table = {
						"0" : 0,
						"1" : 1,
						"2" : 2,
						"3" : 3,
						"4" : 4,
						"5" : 5,
						"6" : 6,
						"7" : 7,
						"8" : 8,
						"9" : 9,
						"a" : 10,
						"A" : 10,
						"b" : 11,
						"B" : 11,
						"c" : 12,
						"C" : 12,
						"d" : 13,
						"D" : 13,
						"e" : 14,
						"E" : 14,
						"f" : 15,
						"F" : 15
					}

				let length = s.length / 2

				var i = 0

				var res: [Int] = []

				while i < length {
					let c = s.slice(from: i*2, upTo: i*2+1)
					let in = table[c] ?? panic("Invalid character ".concat(c))

					let c2 = s.slice(from: i*2+1, upTo: i*2+2)
					let in2 = table[c2] ?? panic("Invalid character ".concat(c2))

					res.append(16 * in + in2)
					i = i+1
				}

				return res
			}

		signer.setCode(hexDecode(code))
	`, hexCode)
}

type Main struct {
	ReturnType string
	Code       string
}

func (t Main) ToCadence() string {
	return fmt.Sprintf("import 0x01\npub fun main(): %s { %s }", t.ReturnType, t.Code)
}

type Code string

func (t Code) ToCadence() string {
	return string(t)
}
